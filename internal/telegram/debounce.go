package telegram

import (
	"context"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// debounceWindow — пауза между поступлениями, после которой воркер сливает
// накопленный буфер в processAssistantTurn. Каждый новый push сбрасывает
// окно. Значение подобрано так, чтобы покрыть «дописал, отправил, начал
// второе» сценарий обычного чата (~800–1500ms между сообщениями), не
// превращая single-message диалог в заметный лаг — UX-сигнал «бот думает»
// показывает typing, который стартует сразу при первом push.
const debounceWindow = 1500 * time.Millisecond

// debounceIdleTimeout — пауза без push'ей, после которой воркер
// самоликвидируется и удаляет себя из map. Активный диалог редко имеет
// паузы такой длины; сто горутин «спят» на ~1MB суммарно — для нашего
// масштаба не проблема.
const debounceIdleTimeout = 5 * time.Minute

// debounceShutdownTimeout — лимит ожидания при graceful-остановке: сколько
// держим WaitGroup, прежде чем сдаться и выйти, теряя in-memory буферы.
// Текущие LLM-вызовы успевают завершиться в типичный таймаут провайдера.
const debounceShutdownTimeout = 360 * time.Second

// debounceBufferSize — размер канала push'ей одного воркера. На обычной
// нагрузке заполняется единицами; при флуде «64 сообщения за окно»
// последующие сообщения дропаются с warn-логом, но процесс продолжает жить.
const debounceBufferSize = 64

// userWorker — приватная per-user сущность, хранящая каналы для подачи
// текста и сигнала drop. Создаётся в submitToDebounce при первом
// сообщении, удаляется из map'ы либо самим воркером по idle, либо
// внешне на shutdown. usersID кладётся при создании — он стабилен,
// пока юзер привязан (а воркер существует только для привязанных).
type userWorker struct {
	ch      chan string
	drop    chan struct{}
	usersID int64
}

// submitToDebounce — единственная точка входа в дебаунс-воркер: и для
// текстовых сообщений, и для расшифрованных голосовых. Если воркер для
// этого telegram_user_id ещё не создан — создаётся и стартует. Push в
// канал — non-blocking; при переполнении (>debounceBufferSize в окне)
// сообщение дропается и пишется warn в лог.
func (t *TelegramBot) submitToDebounce(b *bot.Bot, from *tgmodels.User, chatID, usersID int64, text string) {
	t.debounceMu.Lock()
	defer t.debounceMu.Unlock()

	var w *userWorker = t.debounceWorkers[from.ID]
	if w == nil {
		w = &userWorker{
			ch:      make(chan string, debounceBufferSize),
			drop:    make(chan struct{}, 1),
			usersID: usersID,
		}
		t.debounceWorkers[from.ID] = w
		t.debounceWG.Add(1)
		go t.runUserWorker(t.rootCtx, t.processCtx, b, from, chatID, w)
	}

	select {
	case w.ch <- text:
	default:
		t.logger.Warn("debounce buffer overflow",
			"telegram_user_id", from.ID,
			"buffer_size", debounceBufferSize,
		)
	}
}

// dropUserDebounce — сигнализирует воркеру очистить буфер без обработки
// и остановить typing. Вызывается из command/callback/pending хендлеров
// в момент явного переключения контекста (юзер тапнул /start, кнопку
// меню, callback inline-кнопки и т.п.) — текст в буфере с большой
// вероятностью уже неактуален. No-op, если воркера для этого юзера нет.
func (t *TelegramBot) dropUserDebounce(telegramUserID int64) {
	t.debounceMu.Lock()
	var w *userWorker = t.debounceWorkers[telegramUserID]
	t.debounceMu.Unlock()
	if w == nil {
		return
	}

	select {
	case w.drop <- struct{}{}:
	default:
	}
}

// shutdownDebounce ждёт завершения всех активных воркеров до timeout.
// Воркеры на rootCtx.Done() флашат остаток буфера через processCtx
// (in-flight LLM/SendMessage переживают SIGINT) и выходят сами. При
// срабатывании таймаута зовётся processCancel — это принудительно рубит
// in-flight HTTP-запросы; in-memory буферы и недоставленные ответы
// теряются, это осознанная цена простоты дизайна (см. CLAUDE.md проекта).
// processCancel зовётся и при штатном done — как cleanup, чтобы не
// текли goroutine'ы, держащиеся за processCtx.
func (t *TelegramBot) shutdownDebounce(timeout time.Duration) {
	t.debounceMu.Lock()
	var activeWorkers int = len(t.debounceWorkers)
	t.debounceMu.Unlock()

	t.logger.Info("graceful shutdown: waiting for in-flight turns",
		"workers", activeWorkers,
		"timeout", timeout,
	)

	var done chan struct{} = make(chan struct{})
	go func() {
		t.debounceWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.logger.Info("graceful shutdown: all workers drained")
	case <-time.After(timeout):
		t.logger.Warn("graceful shutdown: timeout, forcing cancel", "timeout", timeout)
	}

	t.processCancel()
}

// runUserWorker — главный цикл per-user воркера. Принимает push'и из
// w.ch, копит в локальном буфере, на каждом push сбрасывает окно
// debounceWindow и idle-таймер. По истечении окна склеивает буфер
// через "\n\n" и зовёт processAssistantTurn (типизированный turn
// уже-привязанного пользователя). Typing запускается при первом push
// в пустой буфер и держится до конца обработки turn'а — даёт юзеру
// сигнал «вижу тебя» сразу после отправки, без задержки на debounceWindow.
//
// Принимает ДВА контекста:
//   - rootCtx  — сигнал «пора выходить» (отменяется на
//     SIGINT/SIGTERM). Используется ТОЛЬКО в select для «case <-rootCtx.Done()».
//   - processCtx (= t.processCtx) — для всех in-flight операций turn'а
//     (LLM-запрос, SendMessage, запись в БД, typing-индикатор). Не отменяется
//     на SIGINT — отменяется только в shutdownDebounce при таймауте.
//     Разделение нужно для graceful-shutdown: на Ctrl+C in-flight LLM-вызов
//     и доставка ответа юзеру переживают сигнал и завершаются штатно.
//
// На w.drop — буфер очищается, typing останавливается, idle-таймер
// продолжает тикать (drop не считается «активностью» юзера, чтобы
// callback-сценарии не продлевали жизнь воркера зря).
//
// На idle.C — самоликвидация под debounceMu (с проверкой, что в w.ch
// не успело прилететь сообщение под локом submit'а — это race между
// «idle сработал» и «push прошёл»; повторная проверка len(w.ch) под
// lock'ом — единственное надёжное место).
//
// На rootCtx.Done() — флашит остаток буфера через processAssistantTurn
// (на processCtx, чтобы LLM/SendMessage не отменялись), затем удаляет
// себя из map и выходит. Если воркер в этот момент уже внутри синхронного
// processAssistantTurn (LLM висит), select не сработает до возврата —
// тогда сначала дотянется текущий turn, потом увидится rootCtx.Done()
// и буфер будет пуст.
func (t *TelegramBot) runUserWorker(rootCtx context.Context, processCtx context.Context, b *bot.Bot, from *tgmodels.User, chatID int64, w *userWorker) {
	defer t.debounceWG.Done()

	var buffer []string
	var typingCancel context.CancelFunc

	var stopTyping = func() {
		if typingCancel != nil {
			typingCancel()
			typingCancel = nil
		}
	}
	defer stopTyping()

	var window *time.Timer = time.NewTimer(debounceWindow)
	window.Stop()
	defer window.Stop()

	var idle *time.Timer = time.NewTimer(debounceIdleTimeout)
	defer idle.Stop()

	for {
		select {
		case <-rootCtx.Done():
			if len(buffer) > 0 {
				var combined string = strings.Join(buffer, "\n\n")
				buffer = nil

				t.processAssistantTurn(processCtx, b, from, chatID, w.usersID, combined)
			}
			stopTyping()

			t.debounceMu.Lock()
			delete(t.debounceWorkers, from.ID)
			t.debounceMu.Unlock()

			return
		case text, ok := <-w.ch:
			if !ok {
				return
			}

			buffer = append(buffer, text)
			window.Reset(debounceWindow)
			idle.Reset(debounceIdleTimeout)

			if typingCancel == nil {
				typingCancel = t.startTyping(processCtx, b, chatID)
			}
		case <-w.drop:
			buffer = nil
			window.Stop()
			stopTyping()
		case <-window.C:
			if len(buffer) == 0 {
				continue
			}

			var combined string = strings.Join(buffer, "\n\n")
			buffer = nil

			t.processAssistantTurn(processCtx, b, from, chatID, w.usersID, combined)
			stopTyping()
			idle.Reset(debounceIdleTimeout)
		case <-idle.C:
			t.debounceMu.Lock()
			if len(w.ch) > 0 {
				t.debounceMu.Unlock()
				idle.Reset(debounceIdleTimeout)

				continue
			}

			delete(t.debounceWorkers, from.ID)
			t.debounceMu.Unlock()

			return
		}
	}
}
