package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/thkx/notification-system/internal/gateway"
	"github.com/thkx/notification-system/internal/storage"
	"github.com/thkx/notification-system/pkg/model"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Server HTTP服务器结构
type Server struct {
	gateway      *gateway.Gateway
	router       *mux.Router
	port         int
	httpServer   *http.Server
	upgrader     websocket.Upgrader
	clients      map[*websocket.Conn]bool
	clientsMutex sync.RWMutex
}

// NewServer 创建一个新的HTTP服务器实例
func NewServer(gateway *gateway.Gateway, port int) *Server {
	r := mux.NewRouter()
	server := &Server{
		gateway: gateway,
		router:  r,
		port:    port,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源的WebSocket连接
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
	server.setupRoutes()
	server.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: server.router,
	}
	return server
}

// setupRoutes 设置API路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")

	// 通知相关API
	notificationRoutes := s.router.PathPrefix("/api/notifications").Subrouter()
	{
		notificationRoutes.HandleFunc("", s.sendNotification).Methods("POST")
		notificationRoutes.HandleFunc("", s.listNotifications).Methods("GET")
		notificationRoutes.HandleFunc("/batch", s.sendBatchNotifications).Methods("POST")
		notificationRoutes.HandleFunc("/{id}", s.getNotificationByID).Methods("GET")
	}

	// WebSocket路由
	s.router.HandleFunc("/ws", s.handleWebSocket).Methods("GET")
}

// Start 启动HTTP服务器
func (s *Server) Start() error {
	log.Printf("HTTP server starting on %s", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown 优雅关闭HTTP服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// healthCheck 健康检查接口
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "notification-system",
	})
}

// sendNotification 发送单个通知的API接口
func (s *Server) sendNotification(w http.ResponseWriter, r *http.Request) {
	var notification model.Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if err := s.gateway.SendNotification(&notification); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// 广播通知给所有WebSocket客户端
	go s.Broadcast(&notification)

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// sendBatchNotifications 批量发送通知的API接口
func (s *Server) sendBatchNotifications(w http.ResponseWriter, r *http.Request) {
	var notifications []*model.Notification
	if err := json.NewDecoder(r.Body).Decode(&notifications); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := s.gateway.SendBatchNotifications(notifications)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	// 广播所有成功的通知给WebSocket客户端（异步处理）
	go func() {
		failedIDs := make(map[string]struct{}, len(result.FailedIDs))
		for _, failedID := range result.FailedIDs {
			failedIDs[failedID] = struct{}{}
		}

		for _, notification := range notifications {
			if notification == nil {
				continue
			}
			if _, failed := failedIDs[notification.ID]; !failed {
				s.Broadcast(notification)
			}
		}
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "success",
		"result": result,
	})
}

// getNotificationByID 查询单条通知状态
func (s *Server) getNotificationByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	notification, err := s.gateway.GetNotificationByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if notification == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "notification not found"})
		return
	}

	writeJSON(w, http.StatusOK, notification)
}

// listNotifications 查询通知列表
func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := storage.NotificationFilter{
		UserID: query.Get("userId"),
		Status: query.Get("status"),
	}

	notifications, err := s.gateway.ListNotifications(filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, notifications)
}

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// 将新客户端添加到客户端映射中
	s.clientsMutex.Lock()
	s.clients[conn] = true
	clientCount := len(s.clients)
	s.clientsMutex.Unlock()

	log.Printf("New WebSocket client connected. Total clients: %d", clientCount)

	// 发送连接成功消息
	welcomeMsg := map[string]string{
		"type":    "welcome",
		"message": "Connected to notification system",
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	}

	// 启动goroutine处理客户端消息
	go s.handleClient(conn)
}

// handleClient 处理单个WebSocket客户端连接
func (s *Server) handleClient(conn *websocket.Conn) {
	defer func() {
		s.removeClient(conn)
		log.Printf("WebSocket client disconnected. Total clients: %d", s.clientCount())
	}()

	// 持续读取客户端消息
	for {
		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 处理客户端消息
		log.Printf("Received message: %v", message)
	}
}

// Broadcast 向所有WebSocket客户端广播通知
func (s *Server) Broadcast(notification *model.Notification) {
	if notification == nil {
		return
	}

	message := map[string]interface{}{
		"type":         "notification",
		"notification": notification,
	}

	clients := s.snapshotClients()

	for _, client := range clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast to client: %v", err)
			s.removeClient(client)
		}
	}

	log.Printf("Broadcasted notification to %d clients", len(clients))
}

func (s *Server) snapshotClients() []*websocket.Conn {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	clients := make([]*websocket.Conn, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}

	return clients
}

func (s *Server) removeClient(conn *websocket.Conn) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	if _, exists := s.clients[conn]; exists {
		delete(s.clients, conn)
		conn.Close()
	}
}

func (s *Server) clientCount() int {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()
	return len(s.clients)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
