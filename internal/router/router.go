package router

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thkx/notification-system/internal/channels"
	"github.com/thkx/notification-system/pkg/errors"
	"github.com/thkx/notification-system/pkg/logger"
	"github.com/thkx/notification-system/pkg/metrics"
	"github.com/thkx/notification-system/pkg/model"
)

// RouterConfig 路由器配置
type RouterConfig struct {
	BufferSize  int           // 队列缓冲大小
	WorkerCount int           // 工作协程数量
	MaxRetries  int           // 队列满时的重试次数
	RetryDelay  time.Duration // 重试延迟基数
}

// Router 通知路由器，负责管理通知渠道和路由通知
type Router struct {
	channels          map[string]channels.Channel // 渠道映射，键为渠道名称，值为渠道实例
	notificationQueue chan *model.Notification    // 通知队列，使用带缓冲的通道
	stopChan          chan struct{}               // 停止信号通道
	config            *RouterConfig               // 路由器配置
	queueSize         atomic.Int64                // 当前队列大小
	processingCount   atomic.Int64                // 当前正在处理的通知数
	stopped           atomic.Bool                 // 是否已停止接收新通知
	workerWg          sync.WaitGroup              // 工作协程等待组
	stopOnce          sync.Once                   // 确保停止逻辑只执行一次
}

func defaultConfig() *RouterConfig {
	return &RouterConfig{
		BufferSize:  1000,
		WorkerCount: 3,
		MaxRetries:  3,
		RetryDelay:  100 * time.Millisecond,
	}
}

func normalizeConfig(cfg *RouterConfig) *RouterConfig {
	if cfg == nil {
		return defaultConfig()
	}

	normalized := *cfg
	if normalized.BufferSize <= 0 {
		normalized.BufferSize = 1000
	}
	if normalized.WorkerCount <= 0 {
		normalized.WorkerCount = 3
	}
	if normalized.MaxRetries < 0 {
		normalized.MaxRetries = 0
	}
	if normalized.RetryDelay <= 0 {
		normalized.RetryDelay = 100 * time.Millisecond
	}

	return &normalized
}

// NewRouter 创建一个新的Router实例（使用默认配置）
func NewRouter() *Router {
	return NewRouterWithConfig(defaultConfig())
}

// NewRouterWithConfig 使用自定义配置创建Router实例
func NewRouterWithConfig(cfg *RouterConfig) *Router {
	cfg = normalizeConfig(cfg)

	router := &Router{
		channels:          make(map[string]channels.Channel),
		notificationQueue: make(chan *model.Notification, cfg.BufferSize),
		stopChan:          make(chan struct{}),
		config:            cfg,
	}

	router.workerWg.Add(cfg.WorkerCount)
	for i := 0; i < cfg.WorkerCount; i++ {
		go router.processQueue()
	}

	return router
}

// RegisterChannel 注册通知渠道
func (r *Router) RegisterChannel(name string, channel channels.Channel) {
	r.channels[name] = channel
}

// RouteNotification 路由通知到相应的渠道，支持队列满时重试
func (r *Router) RouteNotification(notification *model.Notification) error {
	if notification == nil {
		return errors.RouterError("Invalid notification", "notification cannot be nil", nil)
	}

	retryDelay := r.config.RetryDelay
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if err := r.stoppedError(); err != nil {
			return err
		}

		select {
		case <-r.stopChan:
			return r.routerStoppedError(notification.ID)
		case r.notificationQueue <- notification:
			queueSize := r.queueSize.Add(1)
			metrics.GetMetrics().UpdateQueueLength(queueSize)
			return nil
		default:
			if attempt < r.config.MaxRetries {
				exponentialDelay := retryDelay * time.Duration(1<<uint(attempt))
				logger.Warn("Queue full (size=%d), retrying after %v... (attempt %d/%d)",
					r.GetQueueSize(), exponentialDelay, attempt+1, r.config.MaxRetries)
				time.Sleep(exponentialDelay)
				continue
			}

			err := errors.RouterError("Queue full after retries",
				fmt.Sprintf("Notification ID: %s, Queue size: %d, Max retries: %d",
					notification.ID, r.GetQueueSize(), r.config.MaxRetries), nil)
			logger.Error("Failed to route notification %s: %v", notification.ID, err)
			return err
		}
	}

	return nil
}

// processQueue 处理通知队列中的通知
func (r *Router) processQueue() {
	defer r.workerWg.Done()

	for {
		select {
		case <-r.stopChan:
			return
		case notification := <-r.notificationQueue:
			if notification == nil {
				continue
			}

			queueSize := r.queueSize.Add(-1)
			metrics.GetMetrics().UpdateQueueLength(queueSize)

			r.processingCount.Add(1)
			r.processNotification(notification)
			r.processingCount.Add(-1)
		}
	}
}

// processNotification 处理单个通知，并记录性能指标
func (r *Router) processNotification(notification *model.Notification) {
	startTime := time.Now()

	var notificationWg sync.WaitGroup
	for _, channelName := range notification.Channels {
		channel, ok := r.channels[channelName]
		if !ok {
			logger.Warn("Channel not registered: %s for notification %s", channelName, notification.ID)
			continue
		}

		notificationWg.Add(1)
		go func(c channels.Channel, chName string) {
			defer notificationWg.Done()

			if err := c.Send(notification); err != nil {
				logger.Error("Failed to send notification %s via channel %s: %v", notification.ID, chName, err)
			}

			duration := time.Since(startTime).Milliseconds()
			m := metrics.GetMetrics()
			m.AddProcessingTime(duration)

			if m.CheckAlert() {
				logger.Warn("Alert triggered: failure_rate=%.2f%%, queue_util=%.2f%%, avg_time=%dms",
					m.GetFailureRate()*100,
					float64(m.GetQueueLength())/float64(m.MaxQueueLength)*100,
					m.GetProcessingTime())
			}
		}(channel, channelName)
	}

	notificationWg.Wait()
}

// GetQueueSize 获取通知队列的大小
func (r *Router) GetQueueSize() int {
	return int(r.queueSize.Load())
}

func (r *Router) getProcessingCount() int {
	return int(r.processingCount.Load())
}

// Stop 停止路由器，关闭所有工作协程
func (r *Router) Stop() {
	r.markStopped()
	r.closeStopChan()
	r.workerWg.Wait()
}

// GracefulStop 优雅关闭路由器，等待队列处理完成
func (r *Router) GracefulStop(ctx context.Context) error {
	r.markStopped()
	logger.Info("Router graceful stop initiated, queue size: %d", r.GetQueueSize())

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if r.GetQueueSize() == 0 && r.getProcessingCount() == 0 {
			r.closeStopChan()
			r.workerWg.Wait()
			logger.Info("All pending notifications processed, router stopped")
			return nil
		}

		select {
		case <-ctx.Done():
			logger.Warn("Shutdown timeout, queue=%d processing=%d", r.GetQueueSize(), r.getProcessingCount())
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *Router) markStopped() {
	r.stopped.Store(true)
}

func (r *Router) stoppedError() error {
	if r.stopped.Load() {
		return r.routerStoppedError("")
	}
	return nil
}

func (r *Router) routerStoppedError(notificationID string) error {
	details := "router is shutting down"
	if notificationID != "" {
		details = fmt.Sprintf("Notification ID: %s, router is shutting down", notificationID)
	}
	return errors.RouterError("Router stopped", details, nil)
}

func (r *Router) closeStopChan() {
	r.stopOnce.Do(func() {
		close(r.stopChan)
	})
}
