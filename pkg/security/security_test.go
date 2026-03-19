package security

import (
	"testing"
)

func TestValidateNotification(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
		message  string
	}{
		{
			name:     "Empty content",
			content:  "",
			expected: false,
			message:  "Notification content cannot be empty",
		},
		{
			name:     "Content too long",
			content:  generateLongString(1001),
			expected: false,
			message:  "Notification content is too long (maximum 1000 characters)",
		},
		{
			name:     "Content with malicious code",
			content:  "<script>alert('xss')</script>",
			expected: false,
			message:  "Notification content contains malicious code",
		},
		{
			name:     "Content with sensitive info",
			content:  "My phone number is 13812345678",
			expected: false,
			message:  "Notification content contains sensitive information",
		},
		{
			name:     "Valid content",
			content:  "Hello, this is a test notification",
			expected: true,
			message:  "Validation passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateNotification(tt.content)
			if result.Valid != tt.expected {
				t.Errorf("ValidateNotification() Valid = %v, want %v", result.Valid, tt.expected)
			}
			if result.Message != tt.message {
				t.Errorf("ValidateNotification() Message = %v, want %v", result.Message, tt.message)
			}
		})
	}
}

func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Content with HTML",
			content:  "<p>Hello</p>",
			expected: "Hello",
		},
		{
			name:     "Content with JavaScript",
			content:  "javascript:alert('xss')",
			expected: "",
		},
		{
			name:     "Content with credit card",
			content:  "My card is 1234567890123456",
			expected: "My card is 1234********3456",
		},
		{
			name:     "Content with phone number",
			content:  "My phone is 13812345678",
			expected: "My phone is 13812345678",
		},
		{
			name:     "Content with email",
			content:  "My email is test@example.com",
			expected: "My email is tes***@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeContent(tt.content)
			if result != tt.expected {
				t.Errorf("SanitizeContent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func generateLongString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
