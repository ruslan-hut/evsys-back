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
	"time"
)

const (
	apiVersion            = "v1"
	readSystemLogEndpoint = "syslog"
	readBackLogEndpoint   = "backlog"
	userAuthenticate      = "users/authenticate"
	userRegister          = "users/register"
	getChargePoints       = "chp"
	activeTransactions    = "transactions/active"
	transactionInfo       = "transactions/info/:id"
	centralSystemCommand  = "csc"
	wsEndpoint            = "/ws"
)

type Server struct {
	conf         *config.Config
	httpServer   *http.Server
	auth         services.Auth
	statusReader services.StatusReader
	apiHandler   func(ac *Call) ([]byte, int)
	wsHandler    func(request *models.UserRequest) error
	logger       services.LogHandler
	upgrader     websocket.Upgrader
	pool         *Pool
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

func (s *Server) SetAuth(auth services.Auth) {
	s.auth = auth
}

func (s *Server) SetApiHandler(handler func(ac *Call) ([]byte, int)) {
	s.apiHandler = handler
}

func (s *Server) SetWsHandler(handler func(request *models.UserRequest) error) {
	s.wsHandler = handler
}

func (s *Server) SetStatusReader(statusReader services.StatusReader) {
	s.statusReader = statusReader
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
	router.GET(route(activeTransactions), s.activeTransactions)
	router.GET(route(transactionInfo), s.transactionInfo)
	router.OPTIONS("/*path", s.options)
	router.GET(wsEndpoint, s.handleWs)
}

func route(path string) string {
	return fmt.Sprintf("/api/%s/%s", apiVersion, path)
}

func (s *Server) activeTransactions(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ac := &Call{
		CallType: ActiveTransactions,
		Remote:   r.RemoteAddr,
		Token:    s.getToken(r),
	}
	s.handleApiRequest(w, ac)
}

func (s *Server) transactionInfo(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ac := &Call{
		CallType: TransactionInfo,
		Remote:   r.RemoteAddr,
		Token:    s.getToken(r),
		Payload:  []byte(ps.ByName("id")),
	}
	s.handleApiRequest(w, ac)
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
	active     chan *Client
	unregister chan *Client
	clients    map[*Client]bool
	broadcast  chan []byte
	userEvent  chan *models.WsResponse
	logger     services.LogHandler
}

func NewPool() *Pool {
	logger := NewLogger("pool", false)
	return &Pool{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		userEvent:  make(chan *models.WsResponse),
		logger:     logger,
	}
}

func (p *Pool) Start() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
			client.sendResponse(models.Ping, "new connection")
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
				if client.id == message.UserId {
					client.sendResponse(message.Status, message.Info)
				}
			}
		}
	}
}

type Client struct {
	ws             *websocket.Conn
	auth           services.Auth
	statusReader   services.StatusReader
	send           chan []byte
	logger         services.LogHandler
	pool           *Pool
	id             string
	subscription   SubscriptionType
	isClosed       bool
	requestHandler func(request *models.UserRequest) error
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
				c.logger.Error("write message", err)
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				c.logger.Error("read message", err)
			}
			break
		}
		if string(message) == "ping" {
			continue
		}

		var userRequest models.UserRequest
		err = json.Unmarshal(message, &userRequest)
		if err != nil {
			c.logger.Error("read pump: unmarshal", err)
			c.sendResponse(models.Error, "invalid request")
			continue
		}

		if c.auth == nil {
			c.sendResponse(models.Error, "authorization not configured")
			continue
		}

		if userRequest.Token == "" {
			c.sendResponse(models.Error, "token not found")
			continue
		}

		user, err := c.auth.GetUser(userRequest.Token)
		if err != nil {
			c.sendResponse(models.Error, fmt.Sprintf("check token: %v", err))
			continue
		}

		tag, err := c.auth.GetUserTag(user.UserId)
		if err != nil {
			c.sendResponse(models.Error, fmt.Sprintf("get user tag: %v", err))
			continue
		}
		c.id = tag
		userRequest.Token = tag

		err = c.requestHandler(&userRequest)
		if err != nil {
			c.logger.Error("read pump: handle request", err)
			continue
		}

		switch userRequest.Command {
		case models.StartTransaction:
			go c.listenForTransactionStart()
		case models.CheckStatus:
			c.logger.Info(fmt.Sprintf("check status: %v", tag))
		default:
			c.sendResponse(models.Success, "request handled")
		}

	}
}

func (c *Client) listenForTransactionStart() {
	ticker := time.NewTicker(5 * time.Second)
	timeout := time.NewTimer(2 * time.Minute)
	timeStart := time.Now()
	for {
		select {
		case <-ticker.C:
			transaction, err := c.statusReader.GetTransaction(c.id, timeStart)
			if err != nil {
				c.logger.Error("get transaction", err)
				continue
			}
			if transaction.TransactionId > -1 {
				c.sendResponse(models.Success, fmt.Sprintf("transaction started: %v", transaction.TransactionId))
				ticker.Stop()
				timeout.Stop()
				return
			} else {
				c.sendResponse(models.Waiting, "waiting for transaction start")
			}
		case <-timeout.C:
			c.sendResponse(models.Error, "timeout")
			ticker.Stop()
			timeout.Stop()
			return
		}
	}
}

func (c *Client) sendResponse(status models.ResponseStatus, info string) {
	if c.isClosed {
		return
	}
	response := models.WsResponse{
		Status: status,
		Info:   info,
	}
	data, err := json.Marshal(response)
	if err == nil {
		c.send <- data
	} else {
		c.logger.Error("send response", err)
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
		ws:             ws,
		auth:           s.auth,
		statusReader:   s.statusReader,
		send:           make(chan []byte, 256),
		logger:         s.logger,
		pool:           s.pool,
		id:             "",
		subscription:   Broadcast,
		requestHandler: s.wsHandler,
	}
	s.pool.register <- client

	go client.writePump()
	go client.readPump()
}
