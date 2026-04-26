package chatMessagesRepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bklv-kirill/asker/internal/models"
)

type chatMessagesSQLiteRepo struct {
	db *sql.DB
}

// NewChatMessagesSQLiteRepo принимает готовое соединение с SQLite и
// возвращает реализацию Repository. Жизненный цикл соединения управляется
// вызывающим (main.go) — репозиторий его не закрывает.
func NewChatMessagesSQLiteRepo(db *sql.DB) Repository {
	return &chatMessagesSQLiteRepo{db: db}
}

func (r *chatMessagesSQLiteRepo) Create(ctx context.Context, m models.ChatMessageCreate) (int64, error) {
	// Явная конвертация role в string: named-type ChatMessageRole без
	// driver.Valuer некоторые драйверы database/sql напрямую не понимают.
	result, err := r.db.ExecContext(
		ctx,
		`INSERT INTO chat_messages (user_id, role, content) VALUES (?, ?, ?)`,
		m.UserID, string(m.Role), m.Content,
	)
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, errors.Join(ErrCreate, err)
	}

	return id, nil
}

func (r *chatMessagesSQLiteRepo) GetLast(
	ctx context.Context,
	userID int64,
	limit int,
) ([]models.ChatMessage, error) {
	// Тянем DESC + LIMIT — это даёт последние limit строк за один проход
	// по индексу (idx_chat_messages_user_created). Разворот в хронологический
	// порядок (oldest-first) делаем в Go: для LLM удобнее подавать messages
	// в естественном порядке, а лишний ORDER BY ASC на отсортированном
	// срезе — те же байты по дисковому чтению.
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, role, content, created_at
		   FROM chat_messages
		  WHERE user_id = ?
		  ORDER BY created_at DESC, id DESC
		  LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, errors.Join(ErrGetLast, err)
	}
	defer rows.Close()

	var collected []models.ChatMessage
	for rows.Next() {
		var (
			id        int64
			userID    int64
			role      string
			content   string
			createdAt time.Time
		)
		var scanErr error = rows.Scan(&id, &userID, &role, &content, &createdAt)
		if scanErr != nil {
			return nil, errors.Join(ErrGetLast, scanErr)
		}

		collected = append(collected, models.ChatMessage{
			ID:        id,
			UserID:    userID,
			Role:      models.ChatMessageRole(role),
			Content:   content,
			CreatedAt: createdAt,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Join(ErrGetLast, err)
	}

	// Реверс: БД отдала newest-first, наружу нужно oldest-first.
	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	return collected, nil
}
