package router

import (
	"context"
	"fmt"
	"sync"
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
	queueSize         int64                       // 当前队列大小
	mutex             sync.RWMutex                // 互斥锁，用于保护共享资源
}

// NewRouter 创建一个新的Router实例（使用默认配置）
// @return 新创建的Router实例
func NewRouter() *Router {
	defaultCfg := &RouterConfig{
		BufferSize:  1000,
		WorkerCount: 3,
		MaxRetries:  3,
		RetryDelay:  100 * time.Millisecond,
	}
	return NewRouterWithConfig(defaultCfg)
}

// NewRouterWithConfig 使用自定义配置创建Router实例
// @param cfg 路由器配置
// @return 新创建的Router实例
func NewRouterWithConfig(cfg *RouterConfig) *Router {
	if cfg == nil {
		cfg = &RouterConfig{
			BufferSize:  1000,
			WorkerCount: 3,
			MaxRetries:  3,
			RetryDelay:  100 * time.Millisecond,
		}
	}

	router := &Router{
		channels:          make(map[string]channels.Channel),
		notificationQueue: make(chan *model.Notification, cfg.BufferSize),
		stopChan:          make(chan struct{}),
		config:            cfg,
		queueSize:         0,
	}

	// 启动队列处理协程
	for i := 0; i < cfg.WorkerCount; i++ {
		go router.processQueue()
	}

	return router
}

// RegisterChannel 注册通知渠道
// @param name 渠道名称
// @param channel 渠道实例
func (r *Router) RegisterChannel(name string, channel channels.Channel) {
	r.channels[name] = channel
}

// RouteNotification 路由通知到相应的渠道，支持队列满时重试
// @param notification 待路由的通知
// @return 路由过程中的错误
func (r *Router) RouteNotification(notification *model.Notification) error {
	// 尝试多次添加通知到队列，处理队列满的情况
	retryDelay := r.config.RetryDelay

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		select {
		case r.notificationQueue <- notification:
			// 通知成功添加到通道
			r.mutex.Lock()
			r.queueSize++
			queueSize := r.queueSize
			r.mutex.Unlock()

			// 更新队列长度指标
			m := metrics.GetMetrics()
			m.UpdateQueueLength(queueSize)

			return nil
		default:
			// 通道已满
			if attempt < r.config.MaxRetries {
				// 计算退避延迟：初始延迟 * (2^attempt)
				exponentialDelay := retryDelay * time.Duration(1<<uint(attempt))
				logger.Warn("Queue full (size=%d), retrying after %v... (attempt %d/%d)",
					r.GetQueueSize(), exponentialDelay, attempt+1, r.config.MaxRetries)
				time.Sleep(exponentialDelay)
				continue
			}

			// 重试次数耗尽，返回错误
			err := errors.RouterError("Queue full after retries",
				fmt.Sprintf("Notification ID: %s, Queue size: %d, Max retries: %d",
					notification.ID, r.GetQueueSize(), r.config.MaxRetries), nil)
			logger.Error("Failed to route notification %s: %v", notification.ID, err)
			return err
		}
	}

	// 不应该执行到这里
	return nil
}

// processQueue 处理通知队列中的通知
func (r *Router) processQueue() {
	for {
		select {
		case <-r.stopChan:
			return
		case notification := <-r.notificationQueue:
			// 通知成功从通道中取出
			r.mutex.Lock()
			r.queueSize--
			queueSize := r.queueSize
			r.mutex.Unlock()

			// 更新队列长度指标
			metrics := metrics.GetMetrics()
			metrics.UpdateQueueLength(queueSize)

			// 处理通知
			r.processNotification(notification)
		}
	}
}

// processNotification 处理单个通知，并记录性能指标
func (r *Router) processNotification(notification *model.Notification) {
	startTime := time.Now()

	// 遍历通知指定的渠道，发送通知
	for _, channelName := range notification.Channels {
		if channel, ok := r.channels[channelName]; ok {
			// 异步发送通知，记录处理时间
			go func(c channels.Channel, chName string) {
				c.Send(notification)

				// 记录处理时间
				duration := time.Since(startTime).Milliseconds()
				m := metrics.GetMetrics()
				m.AddProcessingTime(duration)

				// 定期检查告警
				if m.CheckAlert() {
					logger.Warn("Alert triggered: failure_rate=%.2f%%, queue_util=%.2f%%, avg_time=%dms",
						m.GetFailureRate()*100,
						float64(m.GetQueueLength())/float64(m.MaxQueueLength)*100,
						m.GetProcessingTime())
				}
			}(channel, channelName)
		}
	}
}

// GetQueueSize 获取通知队列的大小
// @return 队列中通知的数量
func (r *Router) GetQueueSize() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return int(r.queueSize)
}

// Stop 停止路由器，关闭所有工作协程
func (r *Router) Stop() {
	close(r.stopChan)
}

// GracefulStop 优雅关闭路由器，等待队列处理完成
// @param ctx 上下文，用于控制最大等待时间
// @return 如果超时返回错误
func (r *Router) GracefulStop(ctx context.Context) error {
	logger.Info("Router graceful stop initiated, queue size: %d", r.GetQueueSize())

	// 停止接收新通知
	close(r.stopChan)

	// 等待队列清空或超时
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if r.GetQueueSize() > 0 {
				logger.Warn("Shutdown timeout, %d notifications may not be processed", r.GetQueueSize())
				return ctx.Err()
			}
			logger.Info("Router gracefully stopped")
			return nil
		case <-ticker.C:
			if r.GetQueueSize() == 0 {
				logger.Info("All pending notifications processed, router stopped")
				return nil
			}
		}
	}
}
