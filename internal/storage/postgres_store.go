package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

// PostgresStore PostgreSQL实现的通知存储
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore 创建PostgresStore实例
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn cannot be empty")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	store := &PostgresStore{db: db}
	if err := store.ensureSchema(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *PostgresStore) ensureSchema() error {
	query := `
CREATE TABLE IF NOT EXISTS notifications (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	type TEXT,
	content TEXT,
	channels TEXT,
	priority INT,
	scheduled TEXT,
	status TEXT,
	created_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ
);`
	_, err := s.db.Exec(query)
	return err
}

// Save 保存通知
func (s *PostgresStore) Save(notification *model.Notification) error {
	if notification == nil {
		return fmt.Errorf("notification cannot be nil")
	}
	if notification.ID == "" {
		return fmt.Errorf("notification ID cannot be empty")
	}

	now := time.Now().UTC()
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = now
	}
	notification.UpdatedAt = now

	channels := ""
	if len(notification.Channels) > 0 {
		channels = strings.Join(notification.Channels, ",")
	}

	_, err := s.db.ExecContext(context.Background(), `
INSERT INTO notifications (id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO UPDATE SET
	user_id = EXCLUDED.user_id,
	type = EXCLUDED.type,
	content = EXCLUDED.content,
	channels = EXCLUDED.channels,
	priority = EXCLUDED.priority,
	scheduled = EXCLUDED.scheduled,
	status = EXCLUDED.status,
	updated_at = EXCLUDED.updated_at`,
		notification.ID,
		notification.UserID,
		notification.Type,
		notification.Content,
		channels,
		notification.Priority,
		notification.Scheduled,
		notification.Status,
		notification.CreatedAt,
		notification.UpdatedAt,
	)
	return err
}

// UpdateStatus 更新状态
func (s *PostgresStore) UpdateStatus(notificationID string, status string) error {
	if notificationID == "" {
		return fmt.Errorf("notificationID cannot be empty")
	}
	_, err := s.db.ExecContext(context.Background(), `
UPDATE notifications SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now().UTC(), notificationID,
	)
	return err
}

// GetByID 查询通知
func (s *PostgresStore) GetByID(notificationID string) (*model.Notification, error) {
	if notificationID == "" {
		return nil, fmt.Errorf("notificationID cannot be empty")
	}

	n := &model.Notification{}
	var channelsStr sql.NullString
	row := s.db.QueryRowContext(context.Background(), `
SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at
FROM notifications WHERE id=$1`, notificationID)

	if err := row.Scan(&n.ID, &n.UserID, &n.Type, &n.Content, &channelsStr, &n.Priority, &n.Scheduled, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if channelsStr.Valid && channelsStr.String != "" {
		n.Channels = strings.Split(channelsStr.String, ",")
	}

	return n, nil
}

// List 返回符合过滤条件的通知
func (s *PostgresStore) List(filter NotificationFilter) ([]*model.Notification, error) {
	query := "SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at FROM notifications"
	clauses := []string{}
	args := []interface{}{}

	if filter.UserID != "" {
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", len(args)+1))
		args = append(args, filter.UserID)
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, filter.Status)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.Notification, 0)
	for rows.Next() {
		n := &model.Notification{}
		var channelsStr sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Content, &channelsStr, &n.Priority, &n.Scheduled, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		if channelsStr.Valid && channelsStr.String != "" {
			n.Channels = strings.Split(channelsStr.String, ",")
		}
		result = append(result, n)
	}

	return result, rows.Err()
}
