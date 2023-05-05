package internal

import (
	"encoding/json"
	"evsys-back/config"
	"evsys-back/models"
	"evsys-back/services"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	apiVersion            = "v1"
	readSystemLogEndpoint = "syslog"
	readBackLogEndpoint   = "backlog"
	userAuthenticate      = "users/authenticate"
	userRegister          = "users/register"
	getChargePoints       = "chp"
	centralSystemCommand  = "csc"
	wsEndpoint            = "/ws"
)

type Server struct {
	conf       *config.Config
	httpServer *http.Server
	apiHandler func(ac *Call) ([]byte, int)
	logger     services.LogHandler
	upgrader   websocket.Upgrader
	pool       *Pool
}

func NewServer(conf *config.Config) *Server {
	pool := NewPool()
	go pool.Start()

	server := Server{
		conf: conf,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		pool: pool,
	}

	// register itself as a router for httpServer handler
	router := httprouter.New()
	server.Register(router)
	server.httpServer = &http.Server{
		Handler: router,
	}

	return &server
}

func (s *Server) SetApiHandler(handler func(ac *Call) ([]byte, int)) {
	s.apiHandler = handler
}

func (s *Server) SetLogger(logger services.LogHandler) {
	s.logger = logger
}

func (s *Server) Register(router *httprouter.Router) {
	router.GET(route(readSystemLogEndpoint), s.readSystemLog)
	router.GET(route(readBackLogEndpoint), s.readBackLog)
	router.POST(route(userAuthenticate), s.authenticateUser)
	router.POST(route(userRegister), s.registerUser)
	router.POST(route(centralSystemCommand), s.centralSystemCommand)
	router.GET(route(getChargePoints), s.getChargePoints)
	router.OPTIONS("/*path", s.options)
	router.GET(wsEndpoint, s.handleWs)
}

func route(path string) string {
	return fmt.Sprintf("/api/%s/%s", apiVersion, path)
}

func (s *Server) readSystemLog(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ac := &Call{
		CallType: ReadSysLog,
		Remote:   r.RemoteAddr,
		Token:    s.getToken(r),
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) readBackLog(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ac := &Call{
		CallType: ReadBackLog,
		Remote:   r.RemoteAddr,
		Token:    s.getToken(r),
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) authenticateUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("get body while authenticate user", err)
		return
	}
	ac := &Call{
		CallType: AuthenticateUser,
		Remote:   r.RemoteAddr,
		Payload:  body,
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) registerUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("get body while register user", err)
		return
	}
	ac := &Call{
		CallType: RegisterUser,
		Remote:   r.RemoteAddr,
		Payload:  body,
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) centralSystemCommand(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("get body while central system command", err)
		return
	}
	ac := &Call{
		CallType: CentralSystemCommand,
		Token:    s.getToken(r),
		Remote:   r.RemoteAddr,
		Payload:  body,
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) getChargePoints(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ac := &Call{
		CallType: GetChargePoints,
		Remote:   r.RemoteAddr,
		Token:    s.getToken(r),
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) options(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleApiRequest(w http.ResponseWriter, ac *Call) {
	if s.apiHandler != nil {
		data, status := s.apiHandler(ac)
		s.sendApiResponse(w, data, status)
	}
}

func (s *Server) sendApiResponse(w http.ResponseWriter, data []byte, status int) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	if status >= 400 {
		w.WriteHeader(status)
	} else if data == nil {
		w.WriteHeader(http.StatusNoContent)
	} else {
		_, err := w.Write(data)
		if err != nil {
			s.logger.Error("send api response", err)
		}
	}
}

func (s *Server) Start() error {
	if s.conf == nil {
		return fmt.Errorf("configuration not loaded")
	}
	serverAddress := fmt.Sprintf("%s:%s", s.conf.Listen.BindIP, s.conf.Listen.Port)
	s.logger.Info(fmt.Sprintf("starting on %s", serverAddress))
	listener, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return err
	}
	if s.conf.Listen.TLS {
		s.logger.Info("starting https TLS")
		err = s.httpServer.ServeTLS(listener, s.conf.Listen.CertFile, s.conf.Listen.KeyFile)
	} else {
		s.logger.Info("starting http")
		err = s.httpServer.Serve(listener)
	}
	return err
}

func (s *Server) getToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.Contains(header, "Bearer") {
		return strings.Replace(header, "Bearer ", "", 1)
	}
	return ""
}

type SubscriptionType string

const (
	Broadcast SubscriptionType = "broadcast"
	UserEvent SubscriptionType = "user-event"
)

type Pool struct {
	register   chan *Client
	unregister chan *Client
	clients    map[*Client]bool
	broadcast  chan []byte
	userEvent  chan []byte
	logger     services.LogHandler
}

func NewPool() *Pool {
	logger := NewLogger("pool")
	return &Pool{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		userEvent:  make(chan []byte),
		logger:     logger,
	}
}

func (p *Pool) Start() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
			p.logger.Info(fmt.Sprintf("registered %s: total connections: %v", client.ws.RemoteAddr(), len(p.clients)))
		case client := <-p.unregister:
			if _, ok := p.clients[client]; ok {
				delete(p.clients, client)
				close(client.send)
				p.logger.Info(fmt.Sprintf("unregistered %s: total connections: %v", client.ws.RemoteAddr(), len(p.clients)))
			} else {
				p.logger.Warn(fmt.Sprintf("unregistered unknown %s: total connections: %v", client.ws.RemoteAddr(), len(p.clients)))
			}
		case message := <-p.broadcast:
			for client := range p.clients {
				if client.subscription == Broadcast {
					client.send <- message
				}
			}
		case message := <-p.userEvent:
			for client := range p.clients {
				if client.subscription == UserEvent {
					client.send <- message
				}
			}
		}
	}
}

type Client struct {
	ws           *websocket.Conn
	send         chan []byte
	logger       services.LogHandler
	pool         *Pool
	id           string
	subscription SubscriptionType
	isClosed     bool
}

func (c *Client) writePump() {
	defer func() {
		c.close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.ws.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				c.logger.Error(fmt.Sprintf("write message for %s", c.id), err)
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.close()
	}()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				c.logger.Error("read message", err)
			}
			break
		}
		c.logger.Info(fmt.Sprintf("%s --> %s", c.id, message))

		response := models.WsMessage{
			Topic: "pong",
			Data:  string(message),
		}
		data, err := json.Marshal(response)
		if err == nil {
			c.send <- data
			//c.pool.broadcast <- data
		} else {
			c.logger.Error("read pump: marshal response", err)
		}
	}
}

func (c *Client) close() {
	if c.isClosed != true {
		c.isClosed = true
		c.pool.unregister <- c
		_ = c.ws.Close()
	}
}

func (s *Server) handleWs(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("upgrade http to websocket", err)
		return
	}

	client := &Client{
		ws:           ws,
		send:         make(chan []byte, 256),
		logger:       s.logger,
		pool:         s.pool,
		id:           r.RemoteAddr,
		subscription: Broadcast,
	}
	s.pool.register <- client

	go client.writePump()
	go client.readPump()
}
