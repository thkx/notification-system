package storage

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/thkx/notification-system/pkg/model"
)

// NotificationStore 定义通知持久化存储接口
type NotificationFilter struct {
	UserID string
	Status string
	Limit  int
	Offset int
	SortBy string
	Order  string
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

	sortBy, sortOrder := normalizeFilterOptions(filter)
	sort.Slice(matches, func(i, j int) bool {
		left, right := matches[i], matches[j]

		compare := compareNotifications(left, right, sortBy)
		if compare == 0 {
			compare = compareNotifications(left, right, "id")
		}
		if sortOrder == "asc" {
			return compare < 0
		}
		return compare > 0
	})

	start, end := paginate(len(matches), filter.Offset, filter.Limit)

	result := make([]*model.Notification, 0, end-start)
	for _, notification := range matches[start:end] {
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

func normalizeFilterOptions(filter NotificationFilter) (sortBy string, order string) {
	sortBy = strings.ToLower(strings.TrimSpace(filter.SortBy))
	switch sortBy {
	case "createdat", "created_at":
		sortBy = "created_at"
	case "updatedat", "updated_at":
		sortBy = "updated_at"
	case "id":
		sortBy = "id"
	default:
		sortBy = "created_at"
	}

	order = strings.ToLower(strings.TrimSpace(filter.Order))
	if order != "asc" {
		order = "desc"
	}

	return sortBy, order
}

func paginate(total int, offset int, limit int) (start int, end int) {
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	start = offset
	if limit <= 0 {
		return start, total
	}

	end = start + limit
	if end > total {
		end = total
	}
	return start, end
}

func compareNotifications(left, right *model.Notification, sortBy string) int {
	switch sortBy {
	case "updated_at":
		return compareTimes(left.UpdatedAt, right.UpdatedAt)
	case "id":
		return strings.Compare(left.ID, right.ID)
	default:
		return compareTimes(left.CreatedAt, right.CreatedAt)
	}
}

func compareTimes(left, right time.Time) int {
	if left.Before(right) {
		return -1
	}
	if left.After(right) {
		return 1
	}
	return 0
}
