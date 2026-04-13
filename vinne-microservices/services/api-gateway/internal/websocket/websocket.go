package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/randco/randco-microservices/services/api-gateway/internal/metrics"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// Message represents a WebSocket message
type Message struct {
	Type      string          `json:"type"`
	Channel   string          `json:"channel,omitempty"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// Client represents a WebSocket client
type Client struct {
	ID            string
	UserID        string
	Conn          *websocket.Conn
	Send          chan []byte
	Hub           *Hub
	Subscriptions map[string]bool
	mu            sync.RWMutex
}

// Hub maintains active WebSocket connections
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	channels   map[string]map[*Client]bool
	logger     logger.Logger
	metrics    *metrics.Metrics
	mu         sync.RWMutex
	stopCh     chan struct{}
	closed     map[*Client]bool // Track closed clients to prevent double close
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	MaxMessageSize  int64
	WriteTimeout    time.Duration
	PongTimeout     time.Duration
	PingInterval    time.Duration
}

// DefaultWebSocketConfig returns default configuration
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		MaxMessageSize:  512 * 1024, // 512KB
		WriteTimeout:    10 * time.Second,
		PongTimeout:     60 * time.Second,
		PingInterval:    54 * time.Second, // Must be less than PongTimeout
	}
}

// Manager handles WebSocket connections
type Manager struct {
	hub      *Hub
	upgrader websocket.Upgrader
	config   *WebSocketConfig
	logger   logger.Logger
	metrics  *metrics.Metrics
}

// NewManager creates a new WebSocket manager
func NewManager(config *WebSocketConfig, logger logger.Logger, metrics *metrics.Metrics) *Manager {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	hub := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		channels:   make(map[string]map[*Client]bool),
		logger:     logger,
		metrics:    metrics,
		stopCh:     make(chan struct{}),
		closed:     make(map[*Client]bool),
	}

	manager := &Manager{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				// Configure origin checking based on your requirements
				return true
			},
		},
		config:  config,
		logger:  logger,
		metrics: metrics,
	}

	// Start the hub
	go hub.Run()

	return manager
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	defer func() {
		// Clean up all clients on exit
		h.mu.Lock()
		for client := range h.clients {
			if !h.closed[client] {
				close(client.Send)
				h.closed[client] = true
			}
		}
		h.mu.Unlock()
	}()

	for {
		select {
		case <-h.stopCh:
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

			h.logger.Info("WebSocket client connected", "client_id", client.ID, "user_id", client.UserID)
			if h.metrics != nil {
				h.metrics.RecordWebSocketConnection(1)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				// Remove from all channels (not just subscribed ones)
				for channel, clients := range h.channels {
					if _, ok := clients[client]; ok {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.channels, channel)
						}
					}
				}

				// Close send channel only if not already closed
				if !h.closed[client] {
					close(client.Send)
					h.closed[client] = true
				}
			}
			h.mu.Unlock()

			h.logger.Info("WebSocket client disconnected", "client_id", client.ID, "user_id", client.UserID)
			if h.metrics != nil {
				h.metrics.RecordWebSocketConnection(-1)
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			clientsCopy := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				clientsCopy = append(clientsCopy, client)
			}
			h.mu.RUnlock()

			// Process clients without holding the lock
			var toRemove []*Client
			for _, client := range clientsCopy {
				select {
				case client.Send <- message:
				default:
					// Client's send channel is full, mark for removal
					toRemove = append(toRemove, client)
				}
			}

			// Remove dead clients
			if len(toRemove) > 0 {
				h.mu.Lock()
				for _, client := range toRemove {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						// Close send channel only if not already closed
						if !h.closed[client] {
							close(client.Send)
							h.closed[client] = true
						}
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (m *Manager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (assuming authentication middleware has run)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.logger.Error("Failed to upgrade connection", "error", err)
		return
	}

	// Create client
	client := &Client{
		ID:            generateClientID(),
		UserID:        userID,
		Conn:          conn,
		Send:          make(chan []byte, 256),
		Hub:           m.hub,
		Subscriptions: make(map[string]bool),
	}

	// Register client
	m.hub.register <- client

	// Start client goroutines
	go client.writePump(m.config)
	go client.readPump(m.config)
}

// readPump handles incoming messages from the client
func (c *Client) readPump(config *WebSocketConfig) {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(config.MaxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(config.PongTimeout))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(config.PongTimeout))
		return nil
	})

	for {
		var message Message
		err := c.Conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.Hub.logger.Error("WebSocket error", "error", err)
			}
			break
		}

		// Record metric
		if c.Hub.metrics != nil {
			c.Hub.metrics.RecordWebSocketMessage("inbound", message.Type)
		}

		// Handle message based on type
		switch message.Type {
		case "subscribe":
			c.handleSubscribe(message)
		case "unsubscribe":
			c.handleUnsubscribe(message)
		case "ping":
			c.handlePing()
		default:
			// Forward message to appropriate handler
			c.handleMessage(message)
		}
	}
}

// writePump handles outgoing messages to the client
func (c *Client) writePump(config *WebSocketConfig) {
	ticker := time.NewTicker(config.PingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
			if !ok {
				// Channel was closed, send close message
				if err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					c.Hub.logger.Debug("Failed to write close message", "client_id", c.ID, "error", err)
				}
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.Hub.logger.Error("Failed to write message", "client_id", c.ID, "error", err)
				return
			}

			// Record metric
			if c.Hub.metrics != nil {
				c.Hub.metrics.RecordWebSocketMessage("outbound", "data")
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleSubscribe handles channel subscription
func (c *Client) handleSubscribe(message Message) {
	channel := message.Channel
	if channel == "" {
		return
	}

	c.mu.Lock()
	c.Subscriptions[channel] = true
	c.mu.Unlock()

	c.Hub.mu.Lock()
	if c.Hub.channels[channel] == nil {
		c.Hub.channels[channel] = make(map[*Client]bool)
	}
	c.Hub.channels[channel][c] = true
	c.Hub.mu.Unlock()

	// Send confirmation
	response := Message{
		Type:      "subscribed",
		Channel:   channel,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(response)
	if err != nil {
		c.Hub.logger.Error("Failed to marshal subscribe response", "error", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		c.Hub.logger.Warn("Client send buffer full", "client_id", c.ID)
	}
}

// handleUnsubscribe handles channel unsubscription
func (c *Client) handleUnsubscribe(message Message) {
	channel := message.Channel
	if channel == "" {
		return
	}

	c.mu.Lock()
	delete(c.Subscriptions, channel)
	c.mu.Unlock()

	c.Hub.mu.Lock()
	if clients, ok := c.Hub.channels[channel]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(c.Hub.channels, channel)
		}
	}
	c.Hub.mu.Unlock()

	// Send confirmation
	response := Message{
		Type:      "unsubscribed",
		Channel:   channel,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(response)
	if err != nil {
		c.Hub.logger.Error("Failed to marshal unsubscribe response", "error", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		c.Hub.logger.Warn("Client send buffer full", "client_id", c.ID)
	}
}

// handlePing handles ping messages
func (c *Client) handlePing() {
	response := Message{
		Type:      "pong",
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(response)
	if err != nil {
		c.Hub.logger.Error("Failed to marshal ping response", "error", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		c.Hub.logger.Warn("Client send buffer full", "client_id", c.ID)
	}
}

// handleMessage handles generic messages
func (c *Client) handleMessage(message Message) {
	// Process message based on business logic
	// This is where you'd integrate with your backend services

	// Example: Echo the message back
	response := Message{
		Type:      "response",
		Data:      message.Data,
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(response)
	if err != nil {
		c.Hub.logger.Error("Failed to marshal message response", "error", err)
		return
	}

	select {
	case c.Send <- data:
	default:
		c.Hub.logger.Warn("Client send buffer full", "client_id", c.ID)
	}
}

// BroadcastToChannel sends a message to all clients in a channel
func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	clients, exists := h.channels[channel]
	if !exists {
		h.mu.RUnlock()
		return
	}
	// Make a copy to avoid holding lock during send
	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		clientsCopy = append(clientsCopy, client)
	}
	h.mu.RUnlock()

	var toRemove []*Client
	for _, client := range clientsCopy {
		select {
		case client.Send <- message:
		default:
			// Client's send channel is full, mark for removal
			toRemove = append(toRemove, client)
		}
	}

	// Clean up dead clients
	if len(toRemove) > 0 {
		h.mu.Lock()
		for _, client := range toRemove {
			// Remove from global clients map
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// Close send channel only if not already closed
				if !h.closed[client] {
					close(client.Send)
					h.closed[client] = true
				}
			}
			// Remove from all channel subscriptions
			for ch, subs := range h.channels {
				if _, ok := subs[client]; ok {
					delete(subs, client)
					if len(subs) == 0 {
						delete(h.channels, ch)
					}
				}
			}
		}
		h.mu.Unlock()
	}
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Send <- message:
			default:
				// Client's send channel is full
			}
		}
	}
}

// Stop gracefully stops the hub
func (h *Hub) Stop() {
	select {
	case <-h.stopCh:
		// Already stopped
	default:
		close(h.stopCh)
	}
}

// GetConnectedClients returns the number of connected clients
func (h *Hub) GetConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetChannelSubscribers returns the number of subscribers for a channel
func (h *Hub) GetChannelSubscribers(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.channels[channel]; ok {
		return len(clients)
	}
	return 0
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

// SendNotification sends a notification to specific channels or users
func (m *Manager) SendNotification(ctx context.Context, notification Notification) error {
	message := Message{
		Type:      "notification",
		Channel:   notification.Channel,
		Data:      notification.Data,
		Timestamp: time.Now(),
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	if notification.Channel != "" {
		// Send to channel
		m.hub.BroadcastToChannel(notification.Channel, data)
	} else if notification.UserID != "" {
		// Send to specific user
		m.hub.BroadcastToUser(notification.UserID, data)
	} else {
		// Broadcast to all
		m.hub.broadcast <- data
	}

	return nil
}

// Notification represents a notification to be sent
type Notification struct {
	Channel string          `json:"channel,omitempty"`
	UserID  string          `json:"user_id,omitempty"`
	Data    json.RawMessage `json:"data"`
}
