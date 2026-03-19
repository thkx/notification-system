package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorType 错误类型
type ErrorType string

// 定义错误类型常量
const (
	ErrorTypeValidation ErrorType = "validation_error"  // 验证错误
	ErrorTypeChannel    ErrorType = "channel_error"     // 渠道错误
	ErrorTypeDistribution ErrorType = "distribution_error" // 分发错误
	ErrorTypeRouter     ErrorType = "router_error"      // 路由错误
	ErrorTypeGateway    ErrorType = "gateway_error"     // 网关错误
	ErrorTypeService    ErrorType = "service_error"     // 服务错误
	ErrorTypeConfig     ErrorType = "config_error"      // 配置错误
	ErrorTypeInternal   ErrorType = "internal_error"    // 内部错误
)

// AppError 应用错误结构
type AppError struct {
	Type       ErrorType // 错误类型
	Message    string    // 错误消息
	Details    string    // 错误详情
	StatusCode int       // HTTP状态码（如果适用）
	Cause      error     // 原始错误
	Stack      string    // 堆栈信息
}

// Error 实现error接口
func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap 返回原始错误
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New 创建一个新的应用错误
func New(errorType ErrorType, message string, details string, statusCode int, cause error) *AppError {
	// 获取堆栈信息
	stack := getStackInfo()
	
	return &AppError{
		Type:       errorType,
		Message:    message,
		Details:    details,
		StatusCode: statusCode,
		Cause:      cause,
		Stack:      stack,
	}
}

// ValidationError 创建一个验证错误
func ValidationError(message string, details string, cause error) *AppError {
	return New(ErrorTypeValidation, message, details, 400, cause)
}

// ChannelError 创建一个渠道错误
func ChannelError(message string, details string, cause error) *AppError {
	return New(ErrorTypeChannel, message, details, 500, cause)
}

// DistributionError 创建一个分发错误
func DistributionError(message string, details string, cause error) *AppError {
	return New(ErrorTypeDistribution, message, details, 500, cause)
}

// RouterError 创建一个路由错误
func RouterError(message string, details string, cause error) *AppError {
	return New(ErrorTypeRouter, message, details, 500, cause)
}

// GatewayError 创建一个网关错误
func GatewayError(message string, details string, cause error) *AppError {
	return New(ErrorTypeGateway, message, details, 500, cause)
}

// ServiceError 创建一个服务错误
func ServiceError(message string, details string, cause error) *AppError {
	return New(ErrorTypeService, message, details, 500, cause)
}

// ConfigError 创建一个配置错误
func ConfigError(message string, details string, cause error) *AppError {
	return New(ErrorTypeConfig, message, details, 500, cause)
}

// InternalError 创建一个内部错误
func InternalError(message string, details string, cause error) *AppError {
	return New(ErrorTypeInternal, message, details, 500, cause)
}

// getStackInfo 获取堆栈信息
func getStackInfo() string {
	stack := make([]byte, 1024)
	length := runtime.Stack(stack, false)
	stackStr := string(stack[:length])
	
	// 过滤掉当前函数和调用它的函数
	lines := strings.Split(stackStr, "\n")
	if len(lines) > 6 {
		return strings.Join(lines[6:], "\n")
	}
	
	return stackStr
}
