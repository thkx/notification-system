package gateway

import (
	"fmt"

	"github.com/thkx/notification-system/internal/distribution"
	"github.com/thkx/notification-system/internal/storage"
	"github.com/thkx/notification-system/pkg/errors"
	"github.com/thkx/notification-system/pkg/logger"
	"github.com/thkx/notification-system/pkg/model"
	"github.com/thkx/notification-system/pkg/retry"
)

const (
	statusPending    = "pending"
	statusProcessing = "processing"
	statusFailed     = "failed"
	statusDuplicate  = "duplicate"
	statusSent       = "sent"
)

// BatchResult 批量发送结果
type BatchResult struct {
	Total      int      `json:"total"`      // 总通知数
	Successful int      `json:"successful"` // 成功发送数
	Failed     int      `json:"failed"`     // 失败数
	FailedIDs  []string `json:"failedIds"`  // 失败的通知ID列表
	FirstError error    `json:"-"`          // 首个错误
}

// Gateway 通知网关，处理单个和批量通知的发送
type Gateway struct {
	distribution *distribution.Distribution // 分发组件，负责通知的处理和路由
	store        storage.NotificationStore  // 持久化存储接口
}

// NewGateway 创建一个新的Gateway实例
// @param distribution 分发组件实例
// @param store 通知持久化存储
// @return 新创建的Gateway实例
func NewGateway(distribution *distribution.Distribution, store storage.NotificationStore) *Gateway {
	return &Gateway{
		distribution: distribution,
		store:        store,
	}
}

// SendNotification 发送单个通知
// @param notification 待发送的通知
// @return 发送过程中的错误
func (g *Gateway) SendNotification(notification *model.Notification) error {
	if err := validateNotification(notification); err != nil {
		logger.Error("Failed to send notification: %v", err)
		return err
	}

	logger.Info("Sending single notification: ID=%s, UserID=%s, Type=%s",
		notification.ID, notification.UserID, notification.Type)

	if g.store != nil {
		if notification.Status == "" {
			notification.Status = statusPending
		}
		if err := g.store.Save(notification); err != nil {
			logger.Error("Failed to persist notification: ID=%s, Error=%v", notification.ID, err)
		}
		if err := g.store.UpdateStatus(notification.ID, statusProcessing); err != nil {
			logger.Warn("Failed to update notification status to processing: ID=%s, Error=%v", notification.ID, err)
		}
	}

	// 调用分发组件处理通知，使用重试策略避免瞬时故障
	err := retry.Do(func() error {
		return g.distribution.ProcessNotification(notification)
	}, retry.DefaultRetryConfig())
	if err != nil {
		if g.store != nil {
			if updateErr := g.store.UpdateStatus(notification.ID, statusFailed); updateErr != nil {
				logger.Warn("Failed to update notification status to failed: ID=%s, Error=%v", notification.ID, updateErr)
			}
		}

		if appErr, ok := err.(*errors.AppError); ok && appErr.Type == errors.ErrorTypeDistribution && appErr.Message == "Duplicate notification" {
			if g.store != nil {
				if updateErr := g.store.UpdateStatus(notification.ID, statusDuplicate); updateErr != nil {
					logger.Warn("Failed to update notification status to duplicate: ID=%s, Error=%v", notification.ID, updateErr)
				}
			}
			logger.Info("Duplicate notification ignored: ID=%s", notification.ID)
			return nil
		}

		wrappedErr := errors.GatewayError("Failed to send notification", fmt.Sprintf("Notification ID: %s", notification.ID), err)
		logger.Error("Failed to send notification: %v", wrappedErr)
		return wrappedErr
	}

	if g.store != nil {
		if err := g.store.UpdateStatus(notification.ID, statusSent); err != nil {
			logger.Warn("Failed to update notification status to sent: ID=%s, Error=%v", notification.ID, err)
		}
	}

	logger.Info("Notification sent successfully: ID=%s", notification.ID)
	return nil
}

// GetNotificationByID 根据通知ID读取通知状态
func (g *Gateway) GetNotificationByID(notificationID string) (*model.Notification, error) {
	if g.store == nil {
		return nil, fmt.Errorf("no storage configured")
	}
	if notificationID == "" {
		return nil, fmt.Errorf("notificationID cannot be empty")
	}
	return g.store.GetByID(notificationID)
}

func (g *Gateway) ListNotifications(filter storage.NotificationFilter) ([]*model.Notification, error) {
	if g.store == nil {
		return nil, fmt.Errorf("no storage configured")
	}
	return g.store.List(filter)
}

// SendBatchNotifications 批量发送通知，返回详细的批量结果
// @param notifications 待发送的通知列表
// @return 批量发送结果和第一个错误
func (g *Gateway) SendBatchNotifications(notifications []*model.Notification) (*BatchResult, error) {
	logger.Info("Sending batch notifications: Count=%d", len(notifications))

	result := &BatchResult{
		Total:     len(notifications),
		FailedIDs: make([]string, 0),
	}

	// 检查通知列表是否为空
	if len(notifications) == 0 {
		logger.Warn("Empty notification list provided")
		return result, nil
	}

	// 分批处理通知，每批100条
	const batchSize = 100
	for batchStart := 0; batchStart < len(notifications); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(notifications) {
			batchEnd = len(notifications)
		}

		batch := notifications[batchStart:batchEnd]

		// 处理当前批次的通知
		for _, notification := range batch {
			if notification == nil {
				result.Failed++
				logger.Error("Failed to send batch notification: notification is nil")
				if result.FirstError == nil {
					result.FirstError = errors.ValidationError("Invalid notification", "Notification is nil", nil)
				}
				continue
			}

			if err := validateNotification(notification); err != nil {
				result.Failed++
				result.FailedIDs = append(result.FailedIDs, notification.ID)
				logger.Error("Failed to send batch notification: %v", err)
				if result.FirstError == nil {
					result.FirstError = err
				}
				continue
			}

			// 调用网关发送单条通知（包含持久化和状态更新）
			if err := g.SendNotification(notification); err != nil {
				result.Failed++
				result.FailedIDs = append(result.FailedIDs, notification.ID)
				wrappedErr := errors.GatewayError("Failed to send batch notification",
					fmt.Sprintf("Notification ID: %s", notification.ID), err)
				logger.Error("Failed to send batch notification: %v", wrappedErr)
				if result.FirstError == nil {
					result.FirstError = wrappedErr
				}
				continue
			}

			result.Successful++
		}
	}

	// 如果有失败，记录警告日志
	if result.Failed > 0 {
		logger.Warn("Batch operation completed with %d failures out of %d. Failed IDs: %v",
			result.Failed, result.Total, result.FailedIDs)
	} else {
		logger.Info("Batch notifications sent successfully: Count=%d", len(notifications))
	}

	return result, result.FirstError
}

func validateNotification(notification *model.Notification) error {
	if notification == nil {
		return errors.ValidationError("Invalid notification", "Notification cannot be nil", nil)
	}

	if notification.UserID == "" {
		return errors.ValidationError("Invalid notification", "UserID cannot be empty", nil)
	}

	if len(notification.Channels) == 0 {
		return errors.ValidationError("Invalid notification", "At least one channel must be specified", nil)
	}

	return nil
}
