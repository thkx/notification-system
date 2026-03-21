package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ============ 分离的配置结构体 ============

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int      `json:"port"`           // 服务器端口，默认8080
	ReadTimeoutMs  int      `json:"readTimeoutMs"`  // 读超时，默认5000ms
	WriteTimeoutMs int      `json:"writeTimeoutMs"` // 写超时，默认10000ms
	IdleTimeoutMs  int      `json:"idleTimeoutMs"`  // 空闲超时，默认60000ms
	MaxBodyBytes   int64    `json:"maxBodyBytes"`   // 请求体上限，默认1MB
	AllowedOrigins []string `json:"allowedOrigins"` // 允许的跨站WebSocket来源
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	RequireAPIKey bool   `json:"requireApiKey"` // 是否要求API Key
	APIKey        string `json:"apiKey"`        // API Key，可被环境变量覆盖
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
	Security     SecurityConfig     `json:"security"`     // 安全配置
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
	if err := c.Security.Validate(); err != nil {
		return fmt.Errorf("security config validation failed: %w", err)
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

// Validate 验证安全配置
func (s *SecurityConfig) Validate() error {
	if s.RequireAPIKey && strings.TrimSpace(s.APIKey) == "" {
		return fmt.Errorf("requireApiKey is true but apiKey is empty")
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
	if s.ReadTimeoutMs <= 0 {
		return fmt.Errorf("readTimeoutMs must be greater than 0, got %d", s.ReadTimeoutMs)
	}
	if s.WriteTimeoutMs <= 0 {
		return fmt.Errorf("writeTimeoutMs must be greater than 0, got %d", s.WriteTimeoutMs)
	}
	if s.IdleTimeoutMs <= 0 {
		return fmt.Errorf("idleTimeoutMs must be greater than 0, got %d", s.IdleTimeoutMs)
	}
	if s.MaxBodyBytes <= 0 {
		return fmt.Errorf("maxBodyBytes must be greater than 0, got %d", s.MaxBodyBytes)
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

// LoadConfig 加载系统配置
// @return 配置实例
func LoadConfig() (*Config, error) {
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

	return LoadConfigFromFile(configFile)
}

// LoadConfigFromFile 从文件加载配置
// @param filePath 配置文件路径
// @return 配置实例
func LoadConfigFromFile(filePath string) (*Config, error) {
	// 尝试从文件加载配置
	file, err := os.Open(filePath)
	if err == nil {
		defer file.Close()
		config := *getDefaultConfig()
		if decodeErr := json.NewDecoder(file).Decode(&config); decodeErr == nil {
			applyEnvOverrides(&config)
			// 验证配置的合法性
			if err := config.Validate(); err != nil {
				return nil, fmt.Errorf("validate config %s: %w", filePath, err)
			}
			return &config, nil
		} else {
			return nil, fmt.Errorf("decode config %s: %w", filePath, decodeErr)
		}
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("open config %s: %w", filePath, err)
	}

	// 如果文件不存在，返回默认配置
	config := getDefaultConfig()
	applyEnvOverrides(config)
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate default config: %w", err)
	}
	return config, nil
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if apiKey := strings.TrimSpace(os.Getenv("NOTIFICATION_API_KEY")); apiKey != "" {
		cfg.Security.APIKey = apiKey
	}

	if allowedOrigins := strings.TrimSpace(os.Getenv("NOTIFICATION_ALLOWED_ORIGINS")); allowedOrigins != "" {
		parts := strings.Split(allowedOrigins, ",")
		cfg.Server.AllowedOrigins = cfg.Server.AllowedOrigins[:0]
		for _, part := range parts {
			if origin := strings.TrimSpace(part); origin != "" {
				cfg.Server.AllowedOrigins = append(cfg.Server.AllowedOrigins, origin)
			}
		}
	}
}

// getDefaultConfig 获取默认配置
// @return 默认配置实例
func getDefaultConfig() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Port:           8080,    // 默认端口为8080
			ReadTimeoutMs:  5000,    // 默认读超时5秒
			WriteTimeoutMs: 10000,   // 默认写超时10秒
			IdleTimeoutMs:  60000,   // 默认空闲超时60秒
			MaxBodyBytes:   1 << 20, // 默认请求体上限1MB
		},
		Security: SecurityConfig{
			RequireAPIKey: false, // 开发环境默认不强制鉴权
			APIKey:        "",
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
