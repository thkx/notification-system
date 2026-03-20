package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thkx/notification-system/internal/distribution"
	"github.com/thkx/notification-system/internal/gateway"
	"github.com/thkx/notification-system/internal/router"
	"github.com/thkx/notification-system/internal/storage"
	"github.com/thkx/notification-system/pkg/model"
)

// TestBroadcastSingleNotification 测试单个通知的广播
func TestBroadcastSingleNotification(t *testing.T) {
	// 创建测试服务器
	routerCfg := &router.RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  1,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	// 创建通知
	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Test notification",
		Channels: []string{"email"},
	}

	// 编码通知为JSON
	body, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", "/api/notifications", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected success status, got %v", response)
	}
}

// TestBroadcastBatchNotifications 测试批量通知的广播
func TestBroadcastBatchNotifications(t *testing.T) {
	// 创建测试服务器
	routerCfg := &router.RouterConfig{
		BufferSize:  100,
		WorkerCount: 2,
		MaxRetries:  2,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	// 创建批量通知
	notifications := []*model.Notification{
		{
			ID:       "batch-1",
			UserID:   "user-1",
			Type:     "info",
			Content:  "Batch notification 1",
			Channels: []string{"email"},
		},
		{
			ID:       "batch-2",
			UserID:   "user-2",
			Type:     "warning",
			Content:  "Batch notification 2",
			Channels: []string{"sms"},
		},
		{
			ID:       "batch-3",
			UserID:   "user-3",
			Type:     "error",
			Content:  "Batch notification 3",
			Channels: []string{"inapp"},
		},
	}

	// 编码通知为JSON
	body, err := json.Marshal(notifications)
	if err != nil {
		t.Fatalf("Failed to marshal notifications: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", "/api/notifications/batch", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// 验证响应
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected success status, got %v", response)
	}

	// 验证结果包含正确的字段
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid result format")
	}

	if total, ok := result["total"].(float64); !ok || total != 3 {
		t.Errorf("Expected total=3, got %v", result["total"])
	}
}

// TestWebSocketClientManagement 测试WebSocket客户端管理
func TestWebSocketClientManagement(t *testing.T) {
	// 创建测试服务器
	routerCfg := &router.RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  1,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	// 验证初始状态
	if len(server.clients) != 0 {
		t.Errorf("Expected no initial clients, got %d", len(server.clients))
	}
}

// TestBroadcastWithNoClients 测试当没有客户端时的广播
func TestBroadcastWithNoClients(t *testing.T) {
	// 创建测试服务器
	routerCfg := &router.RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  1,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	// 创建通知
	notification := &model.Notification{
		ID:       "test-no-clients",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Test notification",
		Channels: []string{"email"},
	}

	// 广播应该能处理没有客户端的情况
	server.Broadcast(notification)
}

// TestGetNotificationByID 测试根据ID查询通知状态
func TestGetNotificationByID(t *testing.T) {
	routerCfg := &router.RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  1,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	notification := &model.Notification{
		ID:       "lookup-1",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Lookup notification",
		Channels: []string{"email"},
	}

	err := gw.SendNotification(notification)
	if err != nil {
		t.Fatalf("send notification failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/notifications/lookup-1", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var output model.Notification
	if err := json.NewDecoder(w.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if output.ID != "lookup-1" {
		t.Errorf("expected ID lookup-1, got %s", output.ID)
	}
	if output.Status != "sent" {
		t.Errorf("expected status sent, got %s", output.Status)
	}
}

func TestListNotifications(t *testing.T) {
	routerCfg := &router.RouterConfig{BufferSize: 10, WorkerCount: 1, MaxRetries: 1, RetryDelay: 1000}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	server := NewServer(gw, 8080)

	// 先发送两条不同状态的通知
	n1 := &model.Notification{ID: "l1", UserID: "userA", Type: "info", Content: "o1", Channels: []string{"email"}}
	if err := gw.SendNotification(n1); err != nil {
		t.Fatalf("send n1 failed: %v", err)
	}
	
	n2 := &model.Notification{ID: "l2", UserID: "userA", Type: "info", Content: "o2", Channels: []string{"email"}}
	if err := gw.SendNotification(n2); err != nil {
		t.Fatalf("send n2 failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/notifications?userId=userA", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var out []*model.Notification
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(out))
	}

	req2 := httptest.NewRequest("GET", "/api/notifications?userId=userA&status=sent", nil)
	w2 := httptest.NewRecorder()
	server.router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}

	var out2 []*model.Notification
	if err := json.NewDecoder(w2.Body).Decode(&out2); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(out2) != 2 {
		t.Errorf("expected 2 sent notifications, got %d", len(out2))
	}
}

