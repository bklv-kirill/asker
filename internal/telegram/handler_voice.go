package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	telegramUsersRepo "github.com/bklv-kirill/asker/internal/repository/telegram_users"
	"github.com/bklv-kirill/asker/internal/services/stt"
)

var (
	errVoiceGetFile        = errors.New("voice: getFile")
	errVoiceDownload       = errors.New("voice: download")
	errVoiceDownloadStatus = errors.New("voice: download status non-2xx")
)

// handleVoice — обработчик голосовых сообщений (matchMessageVoice). После
// успешной транскрибации вход равен текстовому сообщению — дальше идёт
// общий ассистент-флоу через processAssistantTurn.
//
// Шаги:
//  1. CreateNewTelegramUserIfNotExists — страховка на случай голоса до /start.
//  2. Лимит длительности (sttMaxDurationSec): при превышении — отказ
//     пользователю + выход. STT не дёргается, voice_in не пишется (сообщение
//     по сути не «принято» к обработке).
//  3. Lookup telegram_users + гейт user_id IS NULL: непривязанному юзеру
//     отвечаем «привяжи номер» с inline-кнопкой и выходим — это происходит
//     ДО скачивания аудио и вызова STT, чтобы не жечь квоту провайдера.
//  4. Скачать voice через Bot API (GetFile + FileDownloadLink).
//  5. Транскрибировать через stt.STT. На ошибке/пустом результате — fallback
//     «не получилось распознать», voice_in не пишется.
//  6. voice_in в журнал (Text = расшифровка) — единый event, симметрично
//     message_in для текстового потока.
//  7. processAssistantTurn — общий хвост (профиль/история/LLM/ответ).
func (t *TelegramBot) handleVoice(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Voice == nil {
		return
	}

	t.CreateNewTelegramUserIfNotExists(ctx, update.Message.From)

	var from *tgmodels.User = update.Message.From
	var chatID int64 = update.Message.Chat.ID
	var messageID int64 = int64(update.Message.ID)
	var voice *tgmodels.Voice = update.Message.Voice

	t.logger.Info("incoming voice",
		"chat_id", chatID,
		"telegram_user_id", from.ID,
		"username", from.Username,
		"duration_sec", voice.Duration,
		"file_size", voice.FileSize,
		"mime_type", voice.MimeType,
	)

	if voice.Duration > t.sttMaxDurationSec {
		t.sendAssistantText(ctx, b, from, chatID,
			"⚠️ Голосовое слишком длинное. Сократи или напиши текстом.", nil)

		return
	}

	tgUser, err := t.telegramUsers.GetByTelegramUserID(ctx, from.ID)
	if err != nil {
		if !errors.Is(err, telegramUsersRepo.ErrNotFound) {
			t.logger.Error("telegram_users get on voice", "err", err, "telegram_user_id", from.ID)
		}

		t.sendAssistantText(ctx, b, from, chatID, "❌ Не получилось обработать голосовое. Попробуй позже.", nil)

		return
	}

	if tgUser.UserID == nil {
		t.sendAssistantText(ctx, b, from, chatID, "📱 Чтобы я мог помогать, привяжи номер телефона.", attachPhoneInlineMarkup())

		return
	}

	var stopTyping context.CancelFunc = t.startTyping(ctx, b, chatID)
	defer stopTyping()

	audio, err := t.downloadVoice(ctx, b, voice)
	if err != nil {
		t.logger.Error("voice download", "err", err, "telegram_user_id", from.ID, "file_id", voice.FileID)
		stopTyping()
		t.sendAssistantText(ctx, b, from, chatID, "⚠️ Не получилось загрузить голосовое. Попробуй ещё раз или напиши текстом.", nil)

		return
	}

	transcript, err := t.stt.Transcribe(ctx, audio)
	if err != nil {
		t.logger.Error("stt transcribe", "err", err, "telegram_user_id", from.ID, "file_id", voice.FileID)
		stopTyping()
		t.sendAssistantText(ctx, b, from, chatID, "⚠️ Не получилось распознать голосовое. Попробуй ещё раз или напиши текстом.", nil)

		return
	}

	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		t.logger.Warn("stt empty transcript", "telegram_user_id", from.ID, "file_id", voice.FileID)
		stopTyping()
		t.sendAssistantText(ctx, b, from, chatID, "⚠️ Не расслышал голосовое. Попробуй ещё раз или напиши текстом.", nil)

		return
	}

	t.logger.Info("voice transcribed",
		"chat_id", chatID,
		"telegram_user_id", from.ID,
		"text", transcript,
	)

	t.LogTelegramEvent(ctx, from, telegramEventPayload{
		Event:             eventVoiceIn,
		ChatID:            chatID,
		TelegramMessageID: messageID,
		Text:              transcript,
	})

	t.processAssistantTurn(ctx, b, from, chatID, *tgUser.UserID, transcript)
}

// downloadVoice достаёт voice-файл с серверов Telegram и возвращает его как
// stt.Audio для подачи в STT-провайдер. Шаг 1 — getFile (метаданные с FilePath),
// шаг 2 — HTTP GET на FileDownloadLink (стандартный URL `/file/bot<token>/<path>`).
//
// Voice от Telegram всегда идёт в формате OGG/Opus — хардкодим Filename
// "voice.ogg" и MimeType "audio/ogg" вместо использования voice.MimeType
// (TG иногда присылает "audio/ogg", иногда "audio/ogg; codecs=opus" — последний
// формат провайдер может не распарсить). Расширение в Filename важно для
// Groq/OpenAI: без него возвращается 400 «could not determine file format».
func (t *TelegramBot) downloadVoice(ctx context.Context, b *bot.Bot, voice *tgmodels.Voice) (stt.Audio, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: voice.FileID})
	if err != nil {
		return stt.Audio{}, errors.Join(errVoiceGetFile, err)
	}

	var url string = b.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return stt.Audio{}, errors.Join(errVoiceDownload, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return stt.Audio{}, errors.Join(errVoiceDownload, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return stt.Audio{}, errors.Join(errVoiceDownload, errVoiceDownloadStatus)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stt.Audio{}, errors.Join(errVoiceDownload, err)
	}

	return stt.Audio{
		Bytes:    body,
		MimeType: "audio/ogg",
		Filename: "voice.ogg",
	}, nil
}
