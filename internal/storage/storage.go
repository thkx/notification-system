package storage

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

// NotificationStore 定义通知持久化存储接口
type NotificationFilter struct {
	UserID string
	Status string
}

type NotificationStore interface {
	Save(notification *model.Notification) error
	UpdateStatus(notificationID string, status string) error
	GetByID(notificationID string) (*model.Notification, error)
	List(filter NotificationFilter) ([]*model.Notification, error)
}

// MemoryStore 是一个简单的内存存储实现，用于开发和测试
type MemoryStore struct {
	mutex         sync.RWMutex
	notifications map[string]*model.Notification
}

// NewMemoryStore 创建MemoryStore实例
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[string]*model.Notification),
	}
}

// Save 保存通知记录
func (s *MemoryStore) Save(notification *model.Notification) error {
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

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.notifications[notification.ID] = cloneNotification(notification)
	return nil
}

// UpdateStatus 更新通知状态
func (s *MemoryStore) UpdateStatus(notificationID string, status string) error {
	if notificationID == "" {
		return fmt.Errorf("notificationID cannot be empty")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	notification, exists := s.notifications[notificationID]
	if !exists {
		return fmt.Errorf("notification not found: %s", notificationID)
	}

	notification.Status = status
	notification.UpdatedAt = time.Now().UTC()
	return nil
}

// GetByID 根据ID查询通知
func (s *MemoryStore) GetByID(notificationID string) (*model.Notification, error) {
	if notificationID == "" {
		return nil, fmt.Errorf("notificationID cannot be empty")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	notification, exists := s.notifications[notificationID]
	if !exists {
		return nil, nil
	}

	return cloneNotification(notification), nil
}

// List 返回符合过滤条件的通知
func (s *MemoryStore) List(filter NotificationFilter) ([]*model.Notification, error) {
	s.mutex.RLock()
	matches := make([]*model.Notification, 0, len(s.notifications))

	for _, notification := range s.notifications {
		if filter.UserID != "" && notification.UserID != filter.UserID {
			continue
		}
		if filter.Status != "" && notification.Status != filter.Status {
			continue
		}

		matches = append(matches, notification)
	}
	s.mutex.RUnlock()

	sort.Slice(matches, func(i, j int) bool {
		if !matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].CreatedAt.After(matches[j].CreatedAt)
		}
		if !matches[i].UpdatedAt.Equal(matches[j].UpdatedAt) {
			return matches[i].UpdatedAt.After(matches[j].UpdatedAt)
		}
		return matches[i].ID < matches[j].ID
	})

	result := make([]*model.Notification, 0, len(matches))
	for _, notification := range matches {
		result = append(result, cloneNotification(notification))
	}

	return result, nil
}

func cloneNotification(notification *model.Notification) *model.Notification {
	if notification == nil {
		return nil
	}

	notificationCopy := *notification
	if notification.Channels != nil {
		notificationCopy.Channels = append([]string(nil), notification.Channels...)
	}

	return &notificationCopy
}
