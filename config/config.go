package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ============ 分离的配置结构体 ============

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int `json:"port"` // 服务器端口，默认8080
}

// RouterConfig 路由器配置
type RouterConfig struct {
	BufferSize   int `json:"bufferSize"`   // 队列缓冲大小，默认1000(开发)/5000(生产)
	WorkerCount  int `json:"workerCount"`  // Worker协程数量，默认3(开发)/10(生产)
	MaxRetries   int `json:"maxRetries"`   // 队列满时的重试次数，默认3(开发)/5(生产)
	RetryDelayMs int `json:"retryDelayMs"` // 重试延迟(毫秒)，默认100(开发)/200(生产)
}

// ChannelConfig 单个渠道配置
type ChannelConfig struct {
	Enabled bool `json:"enabled"` // 是否启用该渠道
}

// ChannelsConfig 所有渠道配置
type ChannelsConfig struct {
	Email       ChannelConfig `json:"email"`        // 邮件渠道
	SMS         ChannelConfig `json:"sms"`          // 短信渠道
	InApp       ChannelConfig `json:"inapp"`        // 应用内渠道
	SocialMedia ChannelConfig `json:"social_media"` // 社交媒体渠道
}

// MetricsConfig 性能监控和告警配置
type MetricsConfig struct {
	MaxFailureRate      float64 `json:"maxFailureRate"`      // 最大失败率(0-1)，默认0.2(20%)
	MaxQueueUtilization float64 `json:"maxQueueUtilization"` // 最大队列使用率(0-1)，默认0.8(80%)
	MaxProcessingTime   int64   `json:"maxProcessingTime"`   // 最大处理时间(毫秒)，默认5000ms
}

// DistributionConfig 分发配置
type DistributionConfig struct {
	DeduplicationTTL int `json:"deduplicationTTL"` // 去重缓存TTL(秒)，默认60
}

// StoreConfig 存储配置
type StoreConfig struct {
	Type string `json:"type"` // 存储类型: memory/postgres
	DSN  string `json:"dsn"`  // PostgreSQL 连接字符串，仅当 Type=postgres 时必填
}

// Config 系统总配置结构
type Config struct {
	Server       ServerConfig       `json:"server"`       // 服务器配置
	Router       RouterConfig       `json:"router"`       // 路由器配置
	Channels     ChannelsConfig     `json:"channels"`     // 渠道配置
	Metrics      MetricsConfig      `json:"metrics"`      // 性能监控配置
	Distribution DistributionConfig `json:"distribution"` // 分发配置
	Store        StoreConfig        `json:"store"`        // 存储配置
	Environment  string             `json:"environment"`  // 运行环境
}

// ============ 配置验证方法 ============

// Validate 验证整个配置的合法性
// @return 错误信息，若验证通过则为nil
func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}
	if err := c.Router.Validate(); err != nil {
		return fmt.Errorf("router config validation failed: %w", err)
	}
	if err := c.Metrics.Validate(); err != nil {
		return fmt.Errorf("metrics config validation failed: %w", err)
	}
	if err := c.Distribution.Validate(); err != nil {
		return fmt.Errorf("distribution config validation failed: %w", err)
	}
	if err := c.Store.Validate(); err != nil {
		return fmt.Errorf("storage config validation failed: %w", err)
	}
	return nil
}

// Validate 验证存储配置
func (s *StoreConfig) Validate() error {
	if s.Type == "" {
		s.Type = "memory"
	}
	if s.Type != "memory" && s.Type != "postgres" {
		return fmt.Errorf("unsupported storage type: %s", s.Type)
	}
	if s.Type == "postgres" && s.DSN == "" {
		return fmt.Errorf("postgres storage requires dsn")
	}
	return nil
}

// Validate 验证服务器配置
func (s *ServerConfig) Validate() error {
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 1 and 65535", s.Port)
	}
	return nil
}

// Validate 验证路由器配置
func (r *RouterConfig) Validate() error {
	if r.BufferSize <= 0 {
		return fmt.Errorf("bufferSize must be greater than 0, got %d", r.BufferSize)
	}
	if r.BufferSize > 100000 {
		return fmt.Errorf("bufferSize too large: %d, max 100000", r.BufferSize)
	}

	if r.WorkerCount <= 0 {
		return fmt.Errorf("workerCount must be greater than 0, got %d", r.WorkerCount)
	}
	if r.WorkerCount > 1000 {
		return fmt.Errorf("workerCount too large: %d, max 1000", r.WorkerCount)
	}

	if r.MaxRetries < 0 {
		return fmt.Errorf("maxRetries must be non-negative, got %d", r.MaxRetries)
	}
	if r.MaxRetries > 10 {
		return fmt.Errorf("maxRetries too large: %d, max 10", r.MaxRetries)
	}

	if r.RetryDelayMs < 0 {
		return fmt.Errorf("retryDelayMs must be non-negative, got %d", r.RetryDelayMs)
	}
	if r.RetryDelayMs > 60000 {
		return fmt.Errorf("retryDelayMs too large: %d, max 60000", r.RetryDelayMs)
	}

	return nil
}

// Validate 验证性能监控配置
func (m *MetricsConfig) Validate() error {
	if m.MaxFailureRate < 0 || m.MaxFailureRate > 1 {
		return fmt.Errorf("maxFailureRate must be between 0 and 1, got %f", m.MaxFailureRate)
	}

	if m.MaxQueueUtilization < 0 || m.MaxQueueUtilization > 1 {
		return fmt.Errorf("maxQueueUtilization must be between 0 and 1, got %f", m.MaxQueueUtilization)
	}

	if m.MaxProcessingTime <= 0 {
		return fmt.Errorf("maxProcessingTime must be greater than 0, got %d", m.MaxProcessingTime)
	}
	if m.MaxProcessingTime > 300000 {
		return fmt.Errorf("maxProcessingTime too large: %d, max 300000ms(5min)", m.MaxProcessingTime)
	}

	return nil
}

// Validate 验证分发配置
func (d *DistributionConfig) Validate() error {
	if d.DeduplicationTTL <= 0 {
		return fmt.Errorf("deduplicationTTL must be greater than 0, got %d", d.DeduplicationTTL)
	}
	if d.DeduplicationTTL > 86400 {
		return fmt.Errorf("deduplicationTTL too large: %d, max 86400(1day)", d.DeduplicationTTL)
	}

	return nil
}

// 全局配置实例
var (
	globalConfig *Config
	configMutex  sync.RWMutex
	configPath   string
)

// LoadConfig 加载系统配置
// @return 配置实例
func LoadConfig() *Config {
	// 尝试从环境变量获取配置文件路径
	env := os.Getenv("NOTIFICATION_ENV")
	if env == "" {
		env = "development"
	}

	// 根据环境选择配置文件
	configFile := fmt.Sprintf("config.%s.json", env)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// 如果环境配置文件不存在，使用默认配置文件
		configFile = "config.json"
	}

	configPath = configFile
	return LoadConfigFromFile(configFile)
}

// LoadConfigFromFile 从文件加载配置
// @param filePath 配置文件路径
// @return 配置实例
func LoadConfigFromFile(filePath string) *Config {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 尝试从文件加载配置
	file, err := os.Open(filePath)
	if err == nil {
		defer file.Close()
		var config Config
		if err := json.NewDecoder(file).Decode(&config); err == nil {
			// 验证配置的合法性
			if err := config.Validate(); err != nil {
				fmt.Printf("Config validation failed: %v, using default config\n", err)
				config = *getDefaultConfig()
			}
			globalConfig = &config
			// 启动配置热更新监控
			go monitorConfigChanges(filePath)
			return globalConfig
		}
	}

	// 如果文件加载失败，返回默认配置
	config := getDefaultConfig()
	globalConfig = config
	return config
}

// GetConfig 获取全局配置实例
// @return 全局配置实例
func GetConfig() *Config {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if globalConfig == nil {
		return LoadConfig()
	}
	return globalConfig
}

// monitorConfigChanges 监控配置文件变化
// @param filePath 配置文件路径
func monitorConfigChanges(filePath string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastModTime := getFileModTime(filePath)

	for range ticker.C {
		currentModTime := getFileModTime(filePath)
		if currentModTime.After(lastModTime) {
			// 配置文件发生变化，重新加载
			fmt.Printf("Config file %s changed, reloading...\n", filePath)
			LoadConfigFromFile(filePath)
			lastModTime = currentModTime
		}
	}
}

// getFileModTime 获取文件的修改时间
// @param filePath 文件路径
// @return 文件修改时间
func getFileModTime(filePath string) time.Time {
	info, err := os.Stat(filePath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// getDefaultConfig 获取默认配置
// @return 默认配置实例
func getDefaultConfig() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080, // 默认端口为8080
		},
		Router: RouterConfig{
			BufferSize:   1000, // 默认队列缓冲大小1000
			WorkerCount:  3,    // 默认3个Worker
			MaxRetries:   3,    // 默认重试3次
			RetryDelayMs: 100,  // 默认重试延迟100ms
		},
		Channels: ChannelsConfig{
			Email: ChannelConfig{
				Enabled: true, // 默认启用邮件渠道
			},
			SMS: ChannelConfig{
				Enabled: true, // 默认启用短信渠道
			},
			InApp: ChannelConfig{
				Enabled: true, // 默认启用应用内渠道
			},
			SocialMedia: ChannelConfig{
				Enabled: true, // 默认启用社交媒体渠道
			},
		},
		Metrics: MetricsConfig{
			MaxFailureRate:      0.2,  // 默认最大失败率20%
			MaxQueueUtilization: 0.8,  // 默认最大队列使用率80%
			MaxProcessingTime:   5000, // 默认最大处理时间5000ms
		},
		Distribution: DistributionConfig{
			DeduplicationTTL: 60, // 默认去重缓存TTL 60秒
		},
		Store: StoreConfig{
			Type: "memory", // 默认内存存储
			DSN:  "",
		},
		Environment: "development", // 默认环境为开发环境
	}
	return cfg
}
