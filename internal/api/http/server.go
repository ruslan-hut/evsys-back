package http

import (
	"encoding/json"
	"evsys-back/config"
	"evsys-back/internal"
	"evsys-back/internal/api/handlers/helper"
	"evsys-back/internal/api/handlers/locations"
	"evsys-back/internal/api/handlers/users"
	"evsys-back/internal/api/middleware/authenticate"
	"evsys-back/internal/api/middleware/timeout"
	"evsys-back/internal/lib/sl"
	"evsys-back/models"
	"evsys-back/services"
	"evsys-back/utility"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	apiVersion       = "v1"
	readLog          = "log/{log}"
	getConfig        = "config/{name}"
	userAuthenticate = "users/authenticate"
	userRegister     = "users/register"
	userInfo         = "users/info/{name}"
	usersList        = "users/list"
	generateInvites  = "users/invites"

	getChargePoints       = "chp"
	getChargePointsSearch = "chp/{search}"
	chargePointInfo       = "point/{id}"

	activeTransactions    = "transactions/active"
	transactionInfo       = "transactions/info/{id}"
	transactionList       = "transactions/list"
	transactionListPeriod = "transactions/list/{period}"
	transactionBill       = "transactions/bill"
	locationsList         = "locations"

	centralSystemCommand = "csc"
	wsEndpoint           = "/ws"

	paymentSuccess = "payment/ok"
	paymentFail    = "payment/ko"
	paymentNotify  = "payment/notify"

	paymentMethods    = "payment/methods"
	paymentSaveMethod = "payment/save"
	paymentUpdate     = "payment/update"
	paymentDelete     = "payment/delete"
	paymentSetOrder   = "payment/order"
)

type Server struct {
	conf         *config.Config
	httpServer   *http.Server
	auth         services.Auth
	statusReader services.StatusReader
	apiHandler   func(ac *internal.Call) ([]byte, int)
	wsHandler    func(request *models.UserRequest) error
	payments     services.Payments
	log          *slog.Logger
	upgrader     websocket.Upgrader
	pool         *Pool
}

type Core interface {
	helper.Helper
	authenticate.Authenticate
	users.Users
	locations.Locations
}

func NewServer(conf *config.Config, log *slog.Logger, core Core) *Server {

	server := Server{
		conf: conf,
		log:  log.With(sl.Module("api.server")),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	router := chi.NewRouter()
	router.Use(timeout.Timeout(5))
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/api/v1", func(r chi.Router) {
		// requests with authorization token
		r.Group(func(r chi.Router) {
			r.Use(authenticate.New(log, core))

			r.Get("/locations", locations.ListLocations(log, core))
			r.Get("/chp", locations.ListChargePoints(log, core))
			r.Get("/chp/{search}", locations.ListChargePoints(log, core))
			r.Get("/point/{id}", locations.ChargePointRead(log, core))
			r.Post("/point/{id}", locations.ChargePointSave(log, core))

			r.Get("/users/info/{name}", users.Info(log, core))
			r.Get("/users/list", users.List(log, core))
		})

		// requests without authorization token
		r.Group(func(r chi.Router) {
			r.Get("/config/{name}", helper.Config(log, core))
			r.Post("/users/authenticate", users.Authenticate(log, core))
			//TODO: check usage, why not use auth token?
			r.Post("/users/register", users.Register(log, core))
		})
	})

	server.httpServer = &http.Server{
		Handler: router,
	}

	return &server
}

func (s *Server) SetAuth(auth services.Auth) {
	s.auth = auth
}

func (s *Server) SetApiHandler(handler func(ac *internal.Call) ([]byte, int)) {
	s.apiHandler = handler
}

func (s *Server) SetWsHandler(handler func(request *models.UserRequest) error) {
	s.wsHandler = handler
}

func (s *Server) SetStatusReader(statusReader services.StatusReader) {
	s.statusReader = statusReader
}

func (s *Server) SetPaymentsService(payments services.Payments) {
	s.payments = payments
}

func (s *Server) Register(router *chi.Mux) {

	//router.Get(route(readLog), s.readLog)
	//
	//router.Get(route(generateInvites), s.generateInvites)
	//
	//router.Post(route(centralSystemCommand), s.centralSystemCommand)
	//
	//router.Get(route(activeTransactions), s.activeTransactions)
	//router.Get(route(transactionInfo), s.transactionInfo)
	//router.Get(route(transactionList), s.transactionList)
	//router.Get(route(transactionListPeriod), s.transactionList)
	//router.Get(route(transactionBill), s.transactionBill)
	//
	//router.Get(route(paymentSuccess), s.paymentSuccess)
	//router.Get(route(paymentFail), s.paymentFail)
	//router.Post(route(paymentNotify), s.paymentNotify)
	//router.Get(route(paymentMethods), s.paymentMethods)
	//router.Post(route(paymentSaveMethod), s.paymentSaveMethod)
	//router.Post(route(paymentUpdate), s.paymentUpdateMethod)
	//router.Post(route(paymentDelete), s.paymentDeleteMethod)
	//router.Post(route(paymentSetOrder), s.paymentSetOrder)
	//router.Options("/*path", s.options)
	//router.Get(wsEndpoint, s.handleWs)
}

func route(path string) string {
	return fmt.Sprintf("/api/%s/%s", apiVersion, path)
}

func (s *Server) paymentSaveMethod(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	user := s.authorizeRequest(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	err = s.payments.SavePaymentMethod(user, body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) paymentUpdateMethod(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	user := s.authorizeRequest(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	err = s.payments.UpdatePaymentMethod(user, body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) paymentDeleteMethod(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	user := s.authorizeRequest(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	err = s.payments.DeletePaymentMethod(user, body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) paymentSetOrder(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return
	}
	user := s.authorizeRequest(r)
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	order, err := s.payments.SetOrder(user, body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(order)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) paymentSuccess(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) paymentFail(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	_ = s.payments.Notify([]byte(r.URL.RawQuery))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) paymentNotify(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = s.payments.Notify(body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) options(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Add("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleApiRequest(w http.ResponseWriter, ac *internal.Call) {
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
	} else {

		if status == http.StatusNoContent {
			data = []byte("[]")
		}

		_, err := w.Write(data)
		if err != nil {
			s.log.Error("send api response", err)
		}
	}
}

func (s *Server) Start() error {
	if s.conf == nil {
		return fmt.Errorf("configuration not loaded")
	}

	s.pool = NewPool(s.log)
	go s.pool.Start()

	// start listening for log updates, if update received, send it to all subscribed clients
	go s.listenForUpdates()

	serverAddress := fmt.Sprintf("%s:%s", s.conf.Listen.BindIP, s.conf.Listen.Port)
	s.log.Info(fmt.Sprintf("starting on %s", serverAddress))
	listener, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return err
	}

	if s.conf.Listen.TLS {
		s.log.Info("starting https TLS")
		err = s.httpServer.ServeTLS(listener, s.conf.Listen.CertFile, s.conf.Listen.KeyFile)
	} else {
		s.log.Info("starting http")
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

func (s *Server) authorizeRequest(r *http.Request) *models.User {
	token := s.getToken(r)
	if token == "" {
		return nil
	}
	user, err := s.auth.AuthenticateByToken(token)
	if err != nil {
		s.log.Error("authorize request", err)
		return nil
	}
	return user
}

type SubscriptionType string

const (
	Broadcast        SubscriptionType = "broadcast"
	LogEvent         SubscriptionType = "log-event"
	ChargePointEvent SubscriptionType = "charge-point-event"
)

type Pool struct {
	register   chan *Client
	active     chan *Client
	unregister chan *Client
	clients    map[*Client]bool
	broadcast  chan []byte
	logEvent   chan *models.WsResponse
	chpEvent   chan *models.WsResponse
	logger     *slog.Logger
}

func NewPool(logger *slog.Logger) *Pool {
	//logger := NewLogger("pool", false, nil)
	return &Pool{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		logEvent:   make(chan *models.WsResponse),
		chpEvent:   make(chan *models.WsResponse),
		logger:     logger,
	}
}

func (p *Pool) Start() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
			client.sendResponse(models.Ping, "new connection")
		case client := <-p.unregister:
			if _, ok := p.clients[client]; ok {
				delete(p.clients, client)
				close(client.send)
			} else {
				p.logger.Warn(fmt.Sprintf("pool: unregistered unknown %s: total connections: %v", client.ws.RemoteAddr(), len(p.clients)))
			}
		case message := <-p.broadcast:
			for client := range p.clients {
				if client.subscription == Broadcast {
					client.send <- message
				}
			}
		case message := <-p.logEvent:
			for client := range p.clients {
				if client.subscription == LogEvent {
					client.wsResponse(message)
				}
			}
		case message := <-p.chpEvent:
			for client := range p.clients {
				if client.subscription == ChargePointEvent {
					client.wsResponse(message)
				}
			}
		}
	}
}

type Client struct {
	ws             *websocket.Conn
	user           *models.User
	auth           services.Auth
	statusReader   services.StatusReader // user state holder and transaction state reader
	send           chan []byte           // served by writePump, sending messages to client
	logger         *slog.Logger
	pool           *Pool          // tracking client connect and disconnect, stored active clients array
	id             string         // replaced with idTag after user authorization
	listeners      map[int]string // map of transaction state listeners, key is transaction id, value is user idTag
	subscription   SubscriptionType
	isClosed       bool
	requestHandler func(request *models.UserRequest) error
	mux            *sync.Mutex
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
				c.logger.Warn(fmt.Sprintf("read pump: unexpected close %s", utility.Secret(c.id)))
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

		if c.user == nil {
			c.user, err = c.auth.AuthenticateByToken(userRequest.Token)
			if err != nil {
				c.sendResponse(models.Error, fmt.Sprintf("check token: %v", err))
				continue
			}

			c.id, err = c.auth.GetUserTag(c.user)
			if err != nil {
				c.sendResponse(models.Error, fmt.Sprintf("get user tag: %v", err))
				continue
			}
		}

		userRequest.Token = c.id

		err = c.requestHandler(&userRequest)
		if err != nil {
			c.logger.Error("read pump: handle request", err)
			continue
		}

		switch userRequest.Command {
		case models.StartTransaction:
			timeStart, err := c.statusReader.SaveStatus(c.id, models.StageStart, -1)
			if err == nil {
				go c.listenForTransactionStart(timeStart)
			}
		case models.StopTransaction:
			timeStart, err := c.statusReader.SaveStatus(c.id, models.StageStop, userRequest.TransactionId)
			if err == nil {
				go c.listenForTransactionStop(timeStart, userRequest.TransactionId)
			}
		case models.CheckStatus:
			userState, ok := c.statusReader.GetStatus(c.id)
			if ok {
				c.restoreUserState(userState)
			}
		case models.ListenTransaction:
			_, err := c.statusReader.SaveStatus(c.id, models.StageListen, userRequest.TransactionId)
			if err != nil {
				c.logger.Error("read pump: save status Listen", err)
			}
			_, ok := c.listeners[userRequest.TransactionId]
			if !ok {
				c.mux.Lock()
				c.listeners[userRequest.TransactionId] = c.id
				c.mux.Unlock()
				go c.listenForTransactionState(userRequest.TransactionId)
			}
		case models.StopListenTransaction:
			c.mux.Lock()
			delete(c.listeners, userRequest.TransactionId)
			c.mux.Unlock()
		case models.ListenLog:
			c.setSubscription(LogEvent)
		case models.ListenChargePoints:
			c.setSubscription(ChargePointEvent)
		case models.PingConnection:
			c.sendResponse(models.Success, fmt.Sprintf("ping %s", utility.Secret(c.id)))
		default:
			c.sendResponse(models.Success, "request handled")
		}

	}
}

func (c *Client) setSubscription(subscription SubscriptionType) {
	c.mux.Lock()
	defer c.mux.Unlock()
	c.subscription = subscription
}

func (c *Client) restoreUserState(userState *models.UserStatus) {
	switch userState.Stage {
	case models.StageStart:
		go c.listenForTransactionStart(userState.Time)
	case models.StageStop:
		go c.listenForTransactionStop(userState.Time, userState.TransactionId)
	case models.StageListen:
		_, ok := c.listeners[userState.TransactionId]
		if !ok {
			c.mux.Lock()
			c.listeners[userState.TransactionId] = userState.UserId
			c.mux.Unlock()
			go c.listenForTransactionState(userState.TransactionId)
		}
	}
}

func (c *Client) listenForTransactionStart(timeStart time.Time) {

	maxTimeout := 90
	waitStep := 2

	duration := maxTimeout - int(time.Since(timeStart).Seconds())
	if duration <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)
	pause := time.NewTimer(time.Duration(duration) * time.Second)

	defer func() {
		ticker.Stop()
		pause.Stop()
		if !c.isClosed {
			c.statusReader.ClearStatus(c.id)
		}
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			transaction, err := c.statusReader.GetTransactionAfter(c.id, timeStart)
			if err != nil {
				c.logger.Error("get transaction", err)
				continue
			}
			if transaction.TransactionId > -1 {
				c.wsResponse(&models.WsResponse{
					Status: models.Success,
					Stage:  models.Start,
					Info:   fmt.Sprintf("transaction started: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.wsResponse(&models.WsResponse{
					Status:   models.Waiting,
					Stage:    models.Start,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			c.sendResponse(models.Error, "timeout")
			return
		}
	}
}

func (c *Client) listenForTransactionStop(timeStart time.Time, transactionId int) {

	maxTimeout := 90
	waitStep := 3

	duration := maxTimeout - int(time.Since(timeStart).Seconds())
	if duration <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)
	pause := time.NewTimer(time.Duration(duration) * time.Second)

	defer func() {
		ticker.Stop()
		pause.Stop()
		if !c.isClosed {
			c.statusReader.ClearStatus(c.id)
		}
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			transaction, err := c.statusReader.GetTransaction(transactionId)
			if err != nil {
				c.logger.Error("get transaction", err)
				continue
			}
			if transaction.IsFinished {
				c.wsResponse(&models.WsResponse{
					Status: models.Success,
					Stage:  models.Stop,
					Info:   fmt.Sprintf("transaction stopped: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.wsResponse(&models.WsResponse{
					Status:   models.Waiting,
					Stage:    models.Stop,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			c.sendResponse(models.Error, "timeout")
			return
		}
	}
}

func (c *Client) listenForTransactionState(transactionId int) {
	if transactionId < 0 {
		return
	}

	lastMeterValue := time.Now()
	errorCounter := 0
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			_, ok := c.listeners[transactionId]
			if !ok {
				return
			}
			values, err := c.statusReader.GetLastMeterValues(transactionId, lastMeterValue)
			if err != nil {
				errorCounter++
				if errorCounter > 10 {
					return
				}
				continue
			}
			errorCounter = 0
			for _, value := range values {
				value.Timestamp = value.Time.Unix()
				c.wsResponse(&models.WsResponse{
					Status:          models.Value,
					Stage:           models.Info,
					Info:            value.Unit,
					Progress:        value.Value, // for compatibility with old clients
					Power:           value.Value,
					Price:           value.Price,
					Minute:          value.Minute,
					Id:              transactionId,
					ConnectorId:     value.ConnectorId,
					ConnectorStatus: value.ConnectorStatus,
					MeterValue:      value,
				})
				lastMeterValue = value.Time
				time.Sleep(1 * time.Second)
			}
		}
	}
}

func (c *Client) listenForLogUpdates() {

	lastMessageTime := time.Now()
	//errorCounter := 0
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			messages, err := c.statusReader.ReadLogAfter(lastMessageTime)
			if err != nil {
				//errorCounter++
				//if errorCounter > 10 {
				//	return
				//}
				continue
			}
			if len(messages) > 0 {
				lastMessageTime = messages[len(messages)-1].Timestamp
				for _, message := range messages {
					data, err := json.Marshal(message)
					if err != nil {
						c.logger.Error("marshal message", err)
						continue
					}
					c.wsResponse(&models.WsResponse{
						Status: models.Success,
						Stage:  models.Info,
						Data:   string(data),
					})
				}
			}
		}
	}
}

func (s *Server) listenForUpdates() {

	lastMessageTime := time.Now()
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ticker.C:
			messages, err := s.statusReader.ReadLogAfter(lastMessageTime)
			if err != nil {
				s.log.Error("reading log", err)
				continue
			}
			if len(messages) > 0 {
				lastMessageTime = messages[len(messages)-1].Timestamp
				for _, message := range messages {

					if len(message.ChargePointId) > 1 {
						s.pool.chpEvent <- &models.WsResponse{
							Status: models.Event,
							Stage:  models.ChargePointEvent,
							Data:   message.ChargePointId,
							Info:   message.Text,
						}
					}

					data, err := json.Marshal(message)
					if err != nil {
						s.log.Error("marshal log message", err)
						continue
					}
					s.pool.logEvent <- &models.WsResponse{
						Status: models.Event,
						Stage:  models.LogEvent,
						Data:   string(data),
						Info:   message.Text,
					}

				}
			}
		}
	}
}

func (c *Client) sendResponse(status models.ResponseStatus, info string) {
	response := &models.WsResponse{
		Status: status,
		Info:   info,
		Stage:  models.Info,
	}
	c.wsResponse(response)
}

func (c *Client) wsResponse(response *models.WsResponse) {
	if c.isClosed {
		return
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
		s.log.Error("upgrade http to websocket", err)
		return
	}

	client := &Client{
		ws:             ws,
		auth:           s.auth,
		statusReader:   s.statusReader,
		send:           make(chan []byte, 256),
		logger:         s.log,
		pool:           s.pool,
		id:             "",
		subscription:   ChargePointEvent,
		requestHandler: s.wsHandler,
		listeners:      make(map[int]string),
		mux:            &sync.Mutex{},
	}
	s.pool.register <- client

	go client.writePump()
	go client.readPump()
}
