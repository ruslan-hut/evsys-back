package websocket

import (
	"evsys-back/entity"
	"fmt"
	"log/slog"
)

// PoolClient defines the interface that clients must implement to work with the Pool
type PoolClient interface {
	SendChan() chan []byte
	Subscription() SubscriptionType
	SendResponse(status entity.ResponseStatus, info string)
	WsResponse(response *entity.WsResponse)
	RemoteAddr() string
}

// Pool manages WebSocket client connections and message broadcasting
type Pool struct {
	register   chan PoolClient
	unregister chan PoolClient
	clients    map[PoolClient]bool
	broadcast  chan []byte
	logEvent   chan *entity.WsResponse
	chpEvent   chan *entity.WsResponse
	logger     *slog.Logger
}

// NewPool creates a new WebSocket connection pool
func NewPool(logger *slog.Logger) *Pool {
	return &Pool{
		register:   make(chan PoolClient),
		unregister: make(chan PoolClient),
		clients:    make(map[PoolClient]bool),
		broadcast:  make(chan []byte),
		logEvent:   make(chan *entity.WsResponse),
		chpEvent:   make(chan *entity.WsResponse),
		logger:     logger,
	}
}

// Start begins the pool's event loop for managing client connections and broadcasting messages
func (p *Pool) Start() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
			client.SendResponse(entity.Ping, "new connection")
		case client := <-p.unregister:
			if _, ok := p.clients[client]; ok {
				delete(p.clients, client)
				close(client.SendChan())
			} else {
				p.logger.Warn(fmt.Sprintf("pool: unregistered unknown %s: total connections: %v", client.RemoteAddr(), len(p.clients)))
			}
		case message := <-p.broadcast:
			for client := range p.clients {
				if client.Subscription() == Broadcast {
					client.SendChan() <- message
				}
			}
		case message := <-p.logEvent:
			for client := range p.clients {
				if client.Subscription() == LogEvent {
					client.WsResponse(message)
				}
			}
		case message := <-p.chpEvent:
			for client := range p.clients {
				if client.Subscription() == ChargePointEvent {
					client.WsResponse(message)
				}
			}
		}
	}
}

// Register adds a client to the pool
func (p *Pool) Register(client PoolClient) {
	p.register <- client
}

// Unregister removes a client from the pool
func (p *Pool) Unregister(client PoolClient) {
	p.unregister <- client
}

// SendLogEvent sends a log event to all clients subscribed to LogEvent
func (p *Pool) SendLogEvent(msg *entity.WsResponse) {
	p.logEvent <- msg
}

// SendChpEvent sends a charge point event to all clients subscribed to ChargePointEvent
func (p *Pool) SendChpEvent(msg *entity.WsResponse) {
	p.chpEvent <- msg
}

// Broadcast sends a message to all clients subscribed to Broadcast
func (p *Pool) Broadcast(message []byte) {
	p.broadcast <- message
}
