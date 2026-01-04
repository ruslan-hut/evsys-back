package websocket

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket connection with user state and message handling
type Client struct {
	ws           *websocket.Conn
	user         *entity.User
	core         Core
	statusReader StatusReader
	send         chan []byte
	logger       *slog.Logger
	pool         *Pool
	id           string
	listeners    map[int]string
	subscription SubscriptionType
	isClosed     bool
	mux          sync.Mutex
}

// NewClient creates a new WebSocket client
func NewClient(
	ws *websocket.Conn,
	pool *Pool,
	core Core,
	statusReader StatusReader,
	logger *slog.Logger,
) *Client {
	return &Client{
		ws:           ws,
		core:         core,
		statusReader: statusReader,
		send:         make(chan []byte, 256),
		logger:       logger,
		pool:         pool,
		id:           "",
		subscription: ChargePointEvent,
		listeners:    make(map[int]string),
		mux:          sync.Mutex{},
	}
}

// Start registers the client with the pool and starts the read/write pumps
func (c *Client) Start() {
	c.pool.Register(c)
	go c.writePump()
	go c.readPump()
}

// SendChan returns the client's send channel (implements PoolClient)
func (c *Client) SendChan() chan []byte {
	return c.send
}

// Subscription returns the client's current subscription type (implements PoolClient)
func (c *Client) Subscription() SubscriptionType {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.subscription
}

// RemoteAddr returns the remote address of the WebSocket connection (implements PoolClient)
func (c *Client) RemoteAddr() string {
	return c.ws.RemoteAddr().String()
}

// SendResponse sends a simple response with status and info (implements PoolClient)
func (c *Client) SendResponse(status entity.ResponseStatus, info string) {
	response := &entity.WsResponse{
		Status: status,
		Info:   info,
		Stage:  entity.Info,
	}
	c.WsResponse(response)
}

// WsResponse marshals and sends a response to the client (implements PoolClient)
func (c *Client) WsResponse(response *entity.WsResponse) {
	if c.isClosed {
		return
	}
	data, err := json.Marshal(response)
	if err == nil {
		c.send <- data
	} else {
		c.logger.Error("send response", sl.Err(err))
	}
}

func (c *Client) writePump() {
	defer func() {
		c.close()
	}()
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			c.logger.Error("write message", sl.Err(err))
			return
		}
	}

	_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
}

func (c *Client) readPump() {
	defer func() {
		c.close()
	}()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				c.logger.Warn("read pump: unexpected close")
			}
			break
		}
		if string(message) == "ping" {
			continue
		}

		var userRequest entity.UserRequest
		err = json.Unmarshal(message, &userRequest)
		if err != nil {
			c.logger.Error("read pump: unmarshal", sl.Err(err))
			c.SendResponse(entity.Error, "invalid request")
			continue
		}

		if err = userRequest.Validate(); err != nil {
			c.logger.Error("read pump: validation", sl.Err(err))
			c.SendResponse(entity.Error, fmt.Sprintf("validation: %v", err))
			continue
		}

		if c.user == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.user, err = c.core.AuthenticateByToken(ctx, userRequest.Token)
			cancel()
			if err != nil {
				c.SendResponse(entity.Error, fmt.Sprintf("check token: %v", err))
				continue
			}

			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			c.id, err = c.core.UserTag(ctx, c.user)
			cancel()
			if err != nil {
				c.SendResponse(entity.Error, fmt.Sprintf("get user tag: %v", err))
				continue
			}

			if c.user != nil {
				c.logger = c.logger.With(
					slog.String("user", c.user.Username),
					sl.Secret("id", c.id))
				c.logger.Debug("ws: user authenticated")
			}
		}

		userRequest.Token = c.id

		err = c.core.WsRequest(&userRequest)
		if err != nil {
			c.logger.Error("ws: read pump", sl.Err(err))
			continue
		}

		switch userRequest.Command {
		case entity.StartTransaction:
			timeStart, err := c.statusReader.SaveStatus(c.id, entity.StageStart, -1)
			if err == nil {
				go c.listenForTransactionStart(timeStart)
			}
		case entity.StopTransaction:
			timeStart, err := c.statusReader.SaveStatus(c.id, entity.StageStop, userRequest.TransactionId)
			if err == nil {
				go c.listenForTransactionStop(timeStart, userRequest.TransactionId)
			}
		case entity.CheckStatus:
			userState, ok := c.statusReader.GetStatus(c.id)
			if ok {
				c.restoreUserState(userState)
			}
		case entity.ListenTransaction:
			_, err = c.statusReader.SaveStatus(c.id, entity.StageListen, userRequest.TransactionId)
			if err != nil {
				c.logger.Error("read pump: save status Listen", sl.Err(err))
			}
			_, ok := c.listeners[userRequest.TransactionId]
			if !ok {
				c.mux.Lock()
				c.listeners[userRequest.TransactionId] = c.id
				c.mux.Unlock()
				go c.listenForTransactionState(userRequest.TransactionId)
			}
		case entity.StopListenTransaction:
			c.mux.Lock()
			delete(c.listeners, userRequest.TransactionId)
			c.mux.Unlock()
		case entity.ListenLog:
			c.setSubscription(LogEvent)
		case entity.ListenChargePoints:
			c.setSubscription(ChargePointEvent)
		case entity.PingConnection:
			c.SendResponse(entity.Ping, fmt.Sprintf("pong %s", c.id))
		default:
			c.SendResponse(entity.Error, fmt.Sprintf("unknown command: %s", userRequest.Command))
		}

	}
}

func (c *Client) setSubscription(subscription SubscriptionType) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.subscription = subscription
}

func (c *Client) restoreUserState(userState *entity.UserStatus) {
	switch userState.Stage {
	case entity.StageStart:
		go c.listenForTransactionStart(userState.Time)
	case entity.StageStop:
		go c.listenForTransactionStop(userState.Time, userState.TransactionId)
	case entity.StageListen:
		_, ok := c.listeners[userState.TransactionId]
		if !ok {
			c.mux.Lock()
			c.listeners[userState.TransactionId] = userState.UserId
			c.mux.Unlock()
			go c.listenForTransactionState(userState.TransactionId)
		}
	}
}

func (c *Client) close() {
	if !c.isClosed {
		c.isClosed = true
		c.pool.Unregister(c)
		_ = c.ws.Close()
	}
}

// IsClosed returns whether the client connection is closed
func (c *Client) IsClosed() bool {
	return c.isClosed
}

// ID returns the client's user tag ID
func (c *Client) ID() string {
	return c.id
}

// Listeners returns the map of active transaction listeners
func (c *Client) Listeners() map[int]string {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.listeners
}
