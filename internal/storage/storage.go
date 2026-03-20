package storage

import (
	"fmt"
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
	copy := *notification
	s.notifications[notification.ID] = &copy
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

	notificationCopy := *notification
	return &notificationCopy, nil
}

// List 返回符合过滤条件的通知
func (s *MemoryStore) List(filter NotificationFilter) ([]*model.Notification, error) {
	result := make([]*model.Notification, 0)

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, notification := range s.notifications {
		if filter.UserID != "" && notification.UserID != filter.UserID {
			continue
		}
		if filter.Status != "" && notification.Status != filter.Status {
			continue
		}

		notificationCopy := *notification
		result = append(result, &notificationCopy)
	}

	return result, nil
}
