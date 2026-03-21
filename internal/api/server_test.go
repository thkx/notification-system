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

func newTestServer() *Server {
	routerCfg := &router.RouterConfig{
		BufferSize:  10,
		WorkerCount: 1,
		MaxRetries:  1,
		RetryDelay:  1000,
	}
	notificationRouter := router.NewRouterWithConfig(routerCfg)
	dist := distribution.NewDistribution(notificationRouter)
	gw := gateway.NewGateway(dist, storage.NewMemoryStore())
	return NewServer(gw, 8080)
}

// TestBroadcastSingleNotification 测试单个通知的广播
func TestBroadcastSingleNotification(t *testing.T) {
	server := newTestServer()

	notification := &model.Notification{
		ID:       "test-1",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Test notification",
		Channels: []string{"email"},
	}

	body, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("Failed to marshal notification: %v", err)
	}

	req, err := http.NewRequest("POST", "/api/notifications", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected success status, got %v", response)
	}
}

// TestBroadcastBatchNotifications 测试批量通知的广播
func TestBroadcastBatchNotifications(t *testing.T) {
	server := newTestServer()

	notifications := []*model.Notification{
		{ID: "batch-1", UserID: "user-1", Type: "info", Content: "Batch notification 1", Channels: []string{"email"}},
		{ID: "batch-2", UserID: "user-2", Type: "warning", Content: "Batch notification 2", Channels: []string{"sms"}},
		{ID: "batch-3", UserID: "user-3", Type: "error", Content: "Batch notification 3", Channels: []string{"inapp"}},
	}

	body, err := json.Marshal(notifications)
	if err != nil {
		t.Fatalf("Failed to marshal notifications: %v", err)
	}

	req, err := http.NewRequest("POST", "/api/notifications/batch", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("Expected success status, got %v", response)
	}

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
	server := newTestServer()

	if server.clientCount() != 0 {
		t.Errorf("Expected no initial clients, got %d", server.clientCount())
	}
}

// TestBroadcastWithNoClients 测试当没有客户端时的广播
func TestBroadcastWithNoClients(t *testing.T) {
	server := newTestServer()

	notification := &model.Notification{
		ID:       "test-no-clients",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Test notification",
		Channels: []string{"email"},
	}

	server.Broadcast(notification)
	server.Broadcast(nil)
}

// TestGetNotificationByID 测试根据ID查询通知状态
func TestGetNotificationByID(t *testing.T) {
	server := newTestServer()

	notification := &model.Notification{
		ID:       "lookup-1",
		UserID:   "user-1",
		Type:     "info",
		Content:  "Lookup notification",
		Channels: []string{"email"},
	}

	if err := server.gateway.SendNotification(notification); err != nil {
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

func TestListNotificationsWithFilter(t *testing.T) {
	server := newTestServer()

	notifications := []*model.Notification{
		{ID: "list-1", UserID: "user-1", Type: "info", Content: "one", Channels: []string{"email"}},
		{ID: "list-2", UserID: "user-2", Type: "info", Content: "two", Channels: []string{"email"}},
	}

	for _, notification := range notifications {
		if err := server.gateway.SendNotification(notification); err != nil {
			t.Fatalf("seed notification %s: %v", notification.ID, err)
		}
	}

	req := httptest.NewRequest("GET", "/api/notifications?userId=user-1&status=sent", nil)
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var output []model.Notification
	if err := json.NewDecoder(w.Body).Decode(&output); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(output) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(output))
	}

	if output[0].ID != "list-1" {
		t.Fatalf("expected notification list-1, got %s", output[0].ID)
	}
}

func TestSendBatchNotificationsRejectsInvalidBody(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest("POST", "/api/notifications/batch", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}
