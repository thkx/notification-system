package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/thkx/notification-system/pkg/model"
)

const postgresOperationTimeout = 5 * time.Second

// PostgresStore PostgreSQL实现的通知存储
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore 创建PostgresStore实例
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("dsn cannot be empty")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	store := &PostgresStore{db: db}
	if err := store.ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := store.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

// Close 释放底层数据库连接
func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *PostgresStore) ensureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()

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
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_status_created_at
	ON notifications (user_id, status, created_at DESC, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_status_created_at
	ON notifications (status, created_at DESC, updated_at DESC);
`
	_, err := s.db.ExecContext(ctx, query)
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

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()

	_, err := s.db.ExecContext(ctx, `
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
		serializeChannels(notification.Channels),
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

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()

	result, err := s.db.ExecContext(ctx, `
UPDATE notifications SET status=$1, updated_at=$2 WHERE id=$3`,
		status, time.Now().UTC(), notificationID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found: %s", notificationID)
	}

	return nil
}

// GetByID 查询通知
func (s *PostgresStore) GetByID(notificationID string) (*model.Notification, error) {
	if notificationID == "" {
		return nil, fmt.Errorf("notificationID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()

	n := &model.Notification{}
	var channelsStr sql.NullString
	row := s.db.QueryRowContext(ctx, `
SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at
FROM notifications WHERE id=$1`, notificationID)

	if err := row.Scan(&n.ID, &n.UserID, &n.Type, &n.Content, &channelsStr, &n.Priority, &n.Scheduled, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	n.Channels = deserializeChannels(channelsStr)
	return n, nil
}

// List 返回符合过滤条件的通知
func (s *PostgresStore) List(filter NotificationFilter) ([]*model.Notification, error) {
	query, args := buildNotificationListQuery(filter)

	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, args...)
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
		n.Channels = deserializeChannels(channelsStr)
		result = append(result, n)
	}

	return result, rows.Err()
}

func (s *PostgresStore) ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), postgresOperationTimeout)
	defer cancel()
	return s.db.PingContext(ctx)
}

func buildNotificationListQuery(filter NotificationFilter) (string, []interface{}) {
	query := "SELECT id, user_id, type, content, channels, priority, scheduled, status, created_at, updated_at FROM notifications"
	clauses := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)

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

	query += " ORDER BY created_at DESC, updated_at DESC, id ASC"
	return query, args
}

func serializeChannels(channels []string) string {
	if len(channels) == 0 {
		return ""
	}
	return strings.Join(channels, ",")
}

func deserializeChannels(channels sql.NullString) []string {
	if !channels.Valid || channels.String == "" {
		return nil
	}
	return strings.Split(channels.String, ",")
}
