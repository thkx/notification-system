package security

import (
	"regexp"
	"strings"
)

// ValidationResult 验证结果
type ValidationResult struct {
	Valid   bool   // 是否有效
	Message string // 错误信息
}

// ValidateNotification 验证通知内容
// @param content 通知内容
// @return 验证结果
func ValidateNotification(content string) *ValidationResult {
	// 检查内容长度
	if len(content) == 0 {
		return &ValidationResult{
			Valid:   false,
			Message: "Notification content cannot be empty",
		}
	}

	// 检查内容长度是否过长
	if len(content) > 1000 {
		return &ValidationResult{
			Valid:   false,
			Message: "Notification content is too long (maximum 1000 characters)",
		}
	}

	// 检查是否包含恶意代码
	if containsMaliciousCode(content) {
		return &ValidationResult{
			Valid:   false,
			Message: "Notification content contains malicious code",
		}
	}

	// 检查是否包含敏感信息
	if containsSensitiveInfo(content) {
		return &ValidationResult{
			Valid:   false,
			Message: "Notification content contains sensitive information",
		}
	}

	return &ValidationResult{
		Valid:   true,
		Message: "Validation passed",
	}
}

// containsMaliciousCode 检查是否包含恶意代码
// @param content 通知内容
// @return 是否包含恶意代码
func containsMaliciousCode(content string) bool {
	// 检查HTML标签
	htmlRegex := regexp.MustCompile(`<script[^>]*>.*?</script>`)
	if htmlRegex.MatchString(content) {
		return true
	}

	// 检查JavaScript代码
	jsRegex := regexp.MustCompile(`javascript:[^\s]*`)
	if jsRegex.MatchString(content) {
		return true
	}

	// 检查SQL注入
	sqlRegex := regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE)\s+`)
	if sqlRegex.MatchString(content) {
		return true
	}

	return false
}

// containsSensitiveInfo 检查是否包含敏感信息
// @param content 通知内容
// @return 是否包含敏感信息
func containsSensitiveInfo(content string) bool {
	// 检查信用卡号
	creditCardRegex := regexp.MustCompile(`\b\d{16}\b`)
	if creditCardRegex.MatchString(content) {
		return true
	}

	// 检查身份证号
	idCardRegex := regexp.MustCompile(`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	if idCardRegex.MatchString(content) {
		return true
	}

	// 检查电话号码
	phoneRegex := regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	if phoneRegex.MatchString(content) {
		return true
	}

	// 检查邮箱地址
	emailRegex := regexp.MustCompile(`\b[\w.-]+@[\w.-]+\.\w+\b`)
	if emailRegex.MatchString(content) {
		return true
	}

	return false
}

// SanitizeContent 清理通知内容
// @param content 通知内容
// @return 清理后的内容
func SanitizeContent(content string) string {
	// 移除HTML标签
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	content = htmlRegex.ReplaceAllString(content, "")

	// 移除JavaScript代码
	jsRegex := regexp.MustCompile(`javascript:[^\s]*`)
	content = jsRegex.ReplaceAllString(content, "")

	// 移除SQL注入语句
	sqlRegex := regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE)\s+`)
	content = sqlRegex.ReplaceAllString(content, "")

	// 清理敏感信息
	content = sanitizeSensitiveInfo(content)

	return content
}

// sanitizeSensitiveInfo 清理敏感信息
// @param content 通知内容
// @return 清理后的内容
func sanitizeSensitiveInfo(content string) string {
	// 隐藏信用卡号
	creditCardRegex := regexp.MustCompile(`(\b\d{4})\d{8}(\d{4}\b)`)
	content = creditCardRegex.ReplaceAllString(content, "$1********$2")

	// 隐藏身份证号
	idCardRegex := regexp.MustCompile(`(\b[1-9]\d{5})\d{8}(\d{4}[\dXx]\b)`)
	content = idCardRegex.ReplaceAllString(content, "$1********$2")

	// 隐藏电话号码
	phoneRegex := regexp.MustCompile(`(\b1[3-9]\d{3})\d{4}(\d{4}\b)`)
	content = phoneRegex.ReplaceAllString(content, "$1****$2")

	// 隐藏邮箱地址
	emailRegex := regexp.MustCompile(`(\b[\w.-]+)@([\w.-]+\.\w+\b)`)
	content = emailRegex.ReplaceAllStringFunc(content, func(m string) string {
		parts := strings.Split(m, "@")
		if len(parts) != 2 {
			return m
		}
		username := parts[0]
		if len(username) > 3 {
			username = username[:3] + "***"
		} else {
			username = username + "***"
		}
		return username + "@" + parts[1]
	})

	return content
}
