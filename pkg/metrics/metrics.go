package metrics

import (
	"sort"
	"sync"
	"time"
)

// PercentileMetrics 百分位指标
type PercentileMetrics struct {
	P50 int64 // 中位数（毫秒）
	P95 int64 // 95分位（毫秒）
	P99 int64 // 99分位（毫秒）
}

// AlertConfig 告警配置
type AlertConfig struct {
	MaxFailureRate      float64 // 最大失败率（0-1）
	MaxQueueUtilization float64 // 最大队列使用率（0-1）
	MaxProcessingTime   int64   // 最大处理时间（毫秒）
}

type Metrics struct {
	TotalNotifications      int64
	SuccessfulNotifications int64
	FailedNotifications     int64
	ChannelMetrics          map[string]*ChannelMetric
	QueueLength             int64
	MaxQueueLength          int64        // 队列最大容量
	ProcessingTimes         []int64      // 处理时间列表（毫秒）
	AverageProcessingTime   int64        // 平均处理时间（毫秒）
	FailureRate             float64      // 失败率（0-1）
	AlertConfig             *AlertConfig // 告警配置
	AlertHistory            []Alert      // 告警历史
	mutex                   sync.RWMutex
}

type Alert struct {
	Timestamp   time.Time // 告警时间
	AlertType   string    // 告警类型
	Value       float64   // 告警值
	Threshold   float64   // 告警阈值
	Description string    // 告警描述
}

type ChannelMetric struct {
	Total      int64
	Successful int64
	Failed     int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		ChannelMetrics:  make(map[string]*ChannelMetric),
		ProcessingTimes: make([]int64, 0, 10000),
		MaxQueueLength:  1000,
		AlertHistory:    make([]Alert, 0, 100),
		AlertConfig: &AlertConfig{
			MaxFailureRate:      0.2,  // 20%
			MaxQueueUtilization: 0.8,  // 80%
			MaxProcessingTime:   5000, // 5s
		},
	}
}

// NewMetricsWithConfig 使用自定义告警配置创建Metrics实例
// @param maxFailureRate 最大失败率(0-1)
// @param maxQueueUtilization 最大队列使用率(0-1)
// @param maxProcessingTime 最大处理时间(毫秒)
// @return Metrics实例
func NewMetricsWithConfig(maxFailureRate float64, maxQueueUtilization float64, maxProcessingTime int64, maxQueueLength int64) *Metrics {
	if maxQueueLength <= 0 {
		maxQueueLength = 1000
	}
	return &Metrics{
		ChannelMetrics:  make(map[string]*ChannelMetric),
		ProcessingTimes: make([]int64, 0, 10000),
		MaxQueueLength:  maxQueueLength,
		AlertHistory:    make([]Alert, 0, 100),
		AlertConfig: &AlertConfig{
			MaxFailureRate:      maxFailureRate,
			MaxQueueUtilization: maxQueueUtilization,
			MaxProcessingTime:   maxProcessingTime,
		},
	}
}

func (m *Metrics) IncrementTotal() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.TotalNotifications++
}

func (m *Metrics) IncrementSuccessful() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.SuccessfulNotifications++
}

func (m *Metrics) IncrementFailed() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.FailedNotifications++
}

func (m *Metrics) IncrementChannelTotal(channel string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.ChannelMetrics[channel]; !ok {
		m.ChannelMetrics[channel] = &ChannelMetric{}
	}
	m.ChannelMetrics[channel].Total++
}

func (m *Metrics) IncrementChannelSuccessful(channel string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.ChannelMetrics[channel]; !ok {
		m.ChannelMetrics[channel] = &ChannelMetric{}
	}
	m.ChannelMetrics[channel].Successful++
}

func (m *Metrics) IncrementChannelFailed(channel string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.ChannelMetrics[channel]; !ok {
		m.ChannelMetrics[channel] = &ChannelMetric{}
	}
	m.ChannelMetrics[channel].Failed++
}

func (m *Metrics) GetTotal() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.TotalNotifications
}

func (m *Metrics) GetSuccessful() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.SuccessfulNotifications
}

func (m *Metrics) GetFailed() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.FailedNotifications
}

func (m *Metrics) GetChannelMetrics() map[string]*ChannelMetric {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	// 创建一个副本返回
	result := make(map[string]*ChannelMetric)
	for k, v := range m.ChannelMetrics {
		result[k] = &ChannelMetric{
			Total:      v.Total,
			Successful: v.Successful,
			Failed:     v.Failed,
		}
	}
	return result
}

func (m *Metrics) UpdateQueueLength(length int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.QueueLength = length
}

func (m *Metrics) GetQueueLength() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.QueueLength
}

func (m *Metrics) AddProcessingTime(duration int64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 添加到处理时间列表
	m.ProcessingTimes = append(m.ProcessingTimes, duration)

	// 保持最近10000条记录，防止内存溢出
	if len(m.ProcessingTimes) > 10000 {
		m.ProcessingTimes = m.ProcessingTimes[1:]
	}

	// 更新平均处理时间
	total := int64(0)
	for _, t := range m.ProcessingTimes {
		total += t
	}
	m.AverageProcessingTime = total / int64(len(m.ProcessingTimes))
}

func (m *Metrics) GetProcessingTime() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.AverageProcessingTime
}

// GetPercentiles 获取处理时间百分位
func (m *Metrics) GetPercentiles() *PercentileMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.ProcessingTimes) == 0 {
		return &PercentileMetrics{}
	}

	// 复制并排序
	sorted := make([]int64, len(m.ProcessingTimes))
	copy(sorted, m.ProcessingTimes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	return &PercentileMetrics{
		P50: sorted[len(sorted)/2],
		P95: sorted[int(float64(len(sorted))*0.95)],
		P99: sorted[int(float64(len(sorted))*0.99)],
	}
}

// CheckAlert 检查是否需要告警，并记录告警历史
func (m *Metrics) CheckAlert() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	hasAlert := false

	// 检查失败率是否超过阈值
	if m.TotalNotifications > 0 {
		m.FailureRate = float64(m.FailedNotifications) / float64(m.TotalNotifications)
		if m.FailureRate > m.AlertConfig.MaxFailureRate {
			m.addAlert("FailureRate", m.FailureRate, m.AlertConfig.MaxFailureRate,
				"High failure rate detected")
			hasAlert = true
		}
	}

	// 检查队列利用率是否超过阈值
	if m.MaxQueueLength > 0 {
		queueUtilization := float64(m.QueueLength) / float64(m.MaxQueueLength)
		if queueUtilization > m.AlertConfig.MaxQueueUtilization {
			m.addAlert("QueueUtilization", queueUtilization, m.AlertConfig.MaxQueueUtilization,
				"High queue utilization detected")
			hasAlert = true
		}
	}

	// 检查平均处理时间是否超过阈值
	if m.AverageProcessingTime > m.AlertConfig.MaxProcessingTime {
		m.addAlert("ProcessingTime", float64(m.AverageProcessingTime), float64(m.AlertConfig.MaxProcessingTime),
			"High average processing time detected")
		hasAlert = true
	}

	return hasAlert
}

// addAlert 添加告警记录
func (m *Metrics) addAlert(alertType string, value, threshold float64, description string) {
	alert := Alert{
		Timestamp:   time.Now(),
		AlertType:   alertType,
		Value:       value,
		Threshold:   threshold,
		Description: description,
	}

	m.AlertHistory = append(m.AlertHistory, alert)

	// 保持最近100条告警记录
	if len(m.AlertHistory) > 100 {
		m.AlertHistory = m.AlertHistory[1:]
	}
}

// GetAlerts 获取告警历史
func (m *Metrics) GetAlerts(limit int) []Alert {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if limit <= 0 || limit > len(m.AlertHistory) {
		limit = len(m.AlertHistory)
	}

	result := make([]Alert, limit)
	copy(result, m.AlertHistory[len(m.AlertHistory)-limit:])
	return result
}

// GetFailureRate 获取失败率
func (m *Metrics) GetFailureRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.TotalNotifications == 0 {
		return 0
	}
	return m.FailureRate
}

// 全局指标实例
var globalMetrics = NewMetrics()

// GetMetrics 获取全局指标实例
func GetMetrics() *Metrics {
	return globalMetrics
}

// InitMetricsWithConfig 使用配置初始化全局Metrics实例
// @param maxFailureRate 最大失败率(0-1)
// @param maxQueueUtilization 最大队列使用率(0-1)
// @param maxProcessingTime 最大处理时间(毫秒)
func InitMetricsWithConfig(maxFailureRate float64, maxQueueUtilization float64, maxProcessingTime int64, maxQueueLength int64) {
	globalMetrics = NewMetricsWithConfig(maxFailureRate, maxQueueUtilization, maxProcessingTime, maxQueueLength)
}
