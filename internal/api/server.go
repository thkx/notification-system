package api

import (
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
	upgrader     websocket.Upgrader
	clients      map[*websocket.Conn]bool
	clientsMutex sync.Mutex
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
	serverAddr := fmt.Sprintf(":%d", s.port)
	log.Printf("HTTP server starting on %s", serverAddr)
	return http.ListenAndServe(serverAddr, s.router)
}

// healthCheck 健康检查接口
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "notification-system",
	})
}

// sendNotification 发送单个通知的API接口
func (s *Server) sendNotification(w http.ResponseWriter, r *http.Request) {
	var notification model.Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if err := s.gateway.SendNotification(&notification); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 广播通知给所有WebSocket客户端
	go s.Broadcast(&notification)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// sendBatchNotifications 批量发送通知的API接口
func (s *Server) sendBatchNotifications(w http.ResponseWriter, r *http.Request) {
	var notifications []*model.Notification
	if err := json.NewDecoder(r.Body).Decode(&notifications); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	result, err := s.gateway.SendBatchNotifications(notifications)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	// 广播所有成功的通知给WebSocket客户端（异步处理）
	go func() {
		for _, notification := range notifications {
			// 只广播那些处理成功的通知（通过检查是否在失败ID列表中）
			found := false
			for _, failedID := range result.FailedIDs {
				if notification.ID == failedID {
					found = true
					break
				}
			}
			if !found {
				s.Broadcast(notification)
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"result": result,
	})
}

// getNotificationByID 查询单条通知状态
func (s *Server) getNotificationByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "id is required"})
		return
	}

	notification, err := s.gateway.GetNotificationByID(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if notification == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "notification not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(notification)
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(notifications)
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
	s.clientsMutex.Unlock()

	log.Printf("New WebSocket client connected. Total clients: %d", len(s.clients))

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
		// 从客户端映射中移除并关闭连接
		s.clientsMutex.Lock()
		delete(s.clients, conn)
		s.clientsMutex.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected. Total clients: %d", len(s.clients))
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
	message := map[string]interface{}{
		"type":         "notification",
		"notification": notification,
	}

	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	for client := range s.clients {
		if err := client.WriteJSON(message); err != nil {
			log.Printf("Failed to broadcast to client: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}

	log.Printf("Broadcasted notification to %d clients", len(s.clients))
}
