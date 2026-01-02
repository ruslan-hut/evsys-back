package http

import (
	"context"
	"encoding/json"
	"evsys-back/config"
	"evsys-back/entity"
	centralsystem "evsys-back/internal/api/handlers/central-system"
	"evsys-back/internal/api/handlers/helper"
	"evsys-back/internal/api/handlers/locations"
	"evsys-back/internal/api/handlers/payments"
	"evsys-back/internal/api/handlers/report"
	"evsys-back/internal/api/handlers/transactions"
	"evsys-back/internal/api/handlers/users"
	"evsys-back/internal/api/handlers/usertags"
	"evsys-back/internal/api/middleware/authenticate"
	"evsys-back/internal/api/middleware/timeout"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/gorilla/websocket"
)

type Server struct {
	conf         *config.Config
	httpServer   *http.Server
	core         Core
	statusReader StatusReader
	log          *slog.Logger
	upgrader     websocket.Upgrader
	pool         *Pool
}

type StatusReader interface {
	GetTransactionAfter(ctx context.Context, userId string, after time.Time) (*entity.Transaction, error)
	GetTransaction(ctx context.Context, transactionId int) (*entity.Transaction, error)
	GetLastMeterValues(ctx context.Context, transactionId int, from time.Time) ([]*entity.TransactionMeter, error)

	SaveStatus(userId string, stage entity.Stage, transactionId int) (time.Time, error)
	GetStatus(userId string) (*entity.UserStatus, bool)
	ClearStatus(userId string)

	ReadLogAfter(ctx context.Context, timeStart time.Time) ([]*entity.FeatureMessage, error)
}

type Core interface {
	helper.Helper
	authenticate.Authenticate
	users.Users
	usertags.UserTags
	locations.Locations
	centralsystem.CentralSystem
	transactions.Transactions
	payments.Payments
	report.Reports

	UserTag(ctx context.Context, user *entity.User) (string, error)
	WsRequest(request *entity.UserRequest) error
}

func NewServer(conf *config.Config, log *slog.Logger, core Core) *Server {

	server := Server{
		conf: conf,
		core: core,
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

	router.Use(helper.Options())

	// websocket connection
	router.Route("/", func(r chi.Router) {
		r.Get("/ws", server.handleWs)
	})

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
			r.Post("/users/create", users.Create(log, core))
			r.Put("/users/update/{username}", users.Update(log, core))
			r.Delete("/users/delete/{username}", users.Delete(log, core))
			//router.Get("/users/invites", s.generateInvites)

			r.Get("/user-tags/list", usertags.List(log, core))
			r.Get("/user-tags/info/{idTag}", usertags.Info(log, core))
			r.Post("/user-tags/create", usertags.Create(log, core))
			r.Put("/user-tags/update/{idTag}", usertags.Update(log, core))
			r.Delete("/user-tags/delete/{idTag}", usertags.Delete(log, core))

			r.Post("/csc", centralsystem.Command(log, core))

			r.Get("/transactions/active", transactions.ListActive(log, core))
			r.Get("/transactions/list", transactions.List(log, core))
			r.Get("/transactions/recent", transactions.RecentUserChargePoints(log, core))
			r.Get("/transactions/list/{period}", transactions.List(log, core))
			r.Get("/transactions/info/{id}", transactions.Get(log, core))
			//router.Get("/transactions/bill", s.transactionBill)

			r.Get("/payment/methods", payments.List(log, core))
			r.Post("/payment/save", payments.Save(log, core))
			r.Post("/payment/update", payments.Update(log, core))
			r.Post("/payment/delete", payments.Delete(log, core))
			r.Post("/payment/order", payments.Order(log, core))

			//router.Get("/payment/ok", s.paymentSuccess)
			//router.Get("/payment/ko", s.paymentFail)
			//router.Post("/payment/notify", s.paymentNotify)

			r.Get("/report/month", report.MonthlyStatistics(log, core))
			r.Get("/report/user", report.UsersStatistics(log, core))
			r.Get("/report/charger", report.ChargerStatistics(log, core))

			r.Get("/log/{name}", helper.Log(log, core))
		})

		// requests without authorization token
		r.Group(func(r chi.Router) {
			r.Get("/config/{name}", helper.Config(log, core))
			r.Post("/users/authenticate", users.Authenticate(log, core))
			r.Post("/users/register", users.Register(log, core))
		})
	})

	server.httpServer = &http.Server{
		Handler: router,
	}

	return &server
}

func (s *Server) SetStatusReader(statusReader StatusReader) {
	s.statusReader = statusReader
}

func (s *Server) Start() error {
	if s.conf == nil {
		return fmt.Errorf("configuration not loaded")
	}
	if s.core == nil {
		return fmt.Errorf("core handler not set")
	}

	s.pool = NewPool(s.log)
	go s.pool.Start()

	// start listening for log updates, if update received, send it to all subscribed clients
	go s.listenForUpdates()

	serverAddress := fmt.Sprintf("%s:%s", s.conf.Listen.BindIP, s.conf.Listen.Port)
	s.log.With(
		slog.String("address", serverAddress),
		slog.Bool("tls", s.conf.Listen.TLS),
	).Info("starting server")

	listener, err := net.Listen("tcp", serverAddress)
	if err != nil {
		return err
	}

	if s.conf.Listen.TLS {
		err = s.httpServer.ServeTLS(listener, s.conf.Listen.CertFile, s.conf.Listen.KeyFile)
	} else {
		err = s.httpServer.Serve(listener)
	}

	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

type SubscriptionType string

const (
	Broadcast        SubscriptionType = "broadcast"
	LogEvent         SubscriptionType = "log-event"
	ChargePointEvent SubscriptionType = "charge-point-event"
)

type Pool struct {
	register chan *Client
	//active     chan *Client
	unregister chan *Client
	clients    map[*Client]bool
	broadcast  chan []byte
	logEvent   chan *entity.WsResponse
	chpEvent   chan *entity.WsResponse
	logger     *slog.Logger
}

func NewPool(logger *slog.Logger) *Pool {
	//logger := NewLogger("pool", false, nil)
	return &Pool{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		logEvent:   make(chan *entity.WsResponse),
		chpEvent:   make(chan *entity.WsResponse),
		logger:     logger,
	}
}

func (p *Pool) Start() {
	for {
		select {
		case client := <-p.register:
			p.clients[client] = true
			client.sendResponse(entity.Ping, "new connection")
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
	ws           *websocket.Conn
	user         *entity.User
	core         Core
	statusReader StatusReader // user state holder and transaction state reader
	send         chan []byte  // served by writePump, sending messages to client
	logger       *slog.Logger
	pool         *Pool          // tracking client connect and disconnect, stored active clients array
	id           string         // replaced with idTag after user authorization
	listeners    map[int]string // map of transaction state listeners, key is transaction id, value is user idTag
	subscription SubscriptionType
	isClosed     bool
	mux          sync.Mutex
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
			c.sendResponse(entity.Error, "invalid request")
			continue
		}

		if userRequest.Token == "" {
			c.sendResponse(entity.Error, "token not found")
			continue
		}

		if c.user == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.user, err = c.core.AuthenticateByToken(ctx, userRequest.Token)
			cancel()
			if err != nil {
				c.sendResponse(entity.Error, fmt.Sprintf("check token: %v", err))
				continue
			}

			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			c.id, err = c.core.UserTag(ctx, c.user)
			cancel()
			if err != nil {
				c.sendResponse(entity.Error, fmt.Sprintf("get user tag: %v", err))
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
			c.sendResponse(entity.Ping, fmt.Sprintf("pong %s", c.id))
		default:
			c.sendResponse(entity.Error, fmt.Sprintf("unknown command: %s", userRequest.Command))
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			transaction, err := c.statusReader.GetTransactionAfter(ctx, c.id, timeStart)
			cancel()
			if err != nil {
				c.logger.Error("get transaction", sl.Err(err))
				continue
			}
			if transaction.TransactionId > -1 {
				c.wsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Start,
					Id:     transaction.TransactionId,
					Info:   fmt.Sprintf("transaction started: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.wsResponse(&entity.WsResponse{
					Status:   entity.Waiting,
					Stage:    entity.Start,
					Id:       -1,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			//c.sendResponse(entity.Error, "timeout")
			c.wsResponse(&entity.WsResponse{
				Status: entity.Error,
				Stage:  entity.Start,
				Info:   "timeout",
			})
			return
		}
	}
}

func (c *Client) listenForTransactionStop(timeStart time.Time, transactionId int) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := c.statusReader.GetTransaction(ctx, transactionId)
	cancel()
	if err != nil {
		c.wsResponse(&entity.WsResponse{
			Status: entity.Error,
			Stage:  entity.Stop,
			Info:   fmt.Sprintf("%v", err),
		})
		return
	}

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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			transaction, err := c.statusReader.GetTransaction(ctx, transactionId)
			cancel()
			if err != nil {
				c.logger.Error("get transaction", sl.Err(err))
				continue
			}
			if transaction.IsFinished {
				c.wsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Stop,
					Id:     transaction.TransactionId,
					Info:   fmt.Sprintf("transaction stopped: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.wsResponse(&entity.WsResponse{
					Status:   entity.Waiting,
					Stage:    entity.Stop,
					Id:       transaction.TransactionId,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			//c.sendResponse(entity.Error, "timeout")
			c.wsResponse(&entity.WsResponse{
				Status: entity.Error,
				Stage:  entity.Stop,
				Info:   "timeout",
			})
			return
		}
	}
}

func (c *Client) listenForTransactionState(transactionId int) {
	if transactionId < 0 {
		return
	}

	lastMeterValue := time.Now()
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for range ticker.C {
		if c.isClosed {
			return
		}
		_, ok := c.listeners[transactionId]
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		values, _ := c.statusReader.GetLastMeterValues(ctx, transactionId, lastMeterValue)
		cancel()
		if values == nil {
			continue
		}
		for _, value := range values {
			value.Timestamp = value.Time.Unix()
			c.wsResponse(&entity.WsResponse{
				Status:          entity.Value,
				Stage:           entity.Info,
				Info:            value.Measurand,
				Power:           value.ConsumedEnergy,
				PowerRate:       value.PowerRate,
				SoC:             value.BatteryLevel,
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

func (c *Client) listenForLogUpdates() {

	lastMessageTime := time.Now()
	//errorCounter := 0
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for range ticker.C {
		if c.isClosed {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		messages, _ := c.statusReader.ReadLogAfter(ctx, lastMessageTime)
		cancel()
		if messages == nil {
			continue
		}
		if len(messages) > 0 {
			lastMessageTime = messages[len(messages)-1].Timestamp
			for _, message := range messages {
				data, err := json.Marshal(message)
				if err != nil {
					c.logger.Error("marshal message", sl.Err(err))
					continue
				}
				c.wsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Info,
					Data:   string(data),
				})
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

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		messages, _ := s.statusReader.ReadLogAfter(ctx, lastMessageTime)
		cancel()
		if messages == nil {
			continue
		}
		if len(messages) > 0 {
			lastMessageTime = messages[len(messages)-1].Timestamp
			for _, message := range messages {

				if len(message.ChargePointId) > 1 {
					s.pool.chpEvent <- &entity.WsResponse{
						Status: entity.Event,
						Stage:  entity.ChargePointEvent,
						Data:   message.ChargePointId,
						Info:   message.Text,
					}
				}

				data, err := json.Marshal(message)
				if err != nil {
					s.log.Error("marshal log message", sl.Err(err))
					continue
				}
				s.pool.logEvent <- &entity.WsResponse{
					Status: entity.Event,
					Stage:  entity.LogEvent,
					Data:   string(data),
					Info:   message.Text,
				}

			}
		}
	}
}

func (c *Client) sendResponse(status entity.ResponseStatus, info string) {
	response := &entity.WsResponse{
		Status: status,
		Info:   info,
		Stage:  entity.Info,
	}
	c.wsResponse(response)
}

func (c *Client) wsResponse(response *entity.WsResponse) {
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

func (c *Client) close() {
	if !c.isClosed {
		c.isClosed = true
		c.pool.unregister <- c
		_ = c.ws.Close()
	}
}

func (s *Server) handleWs(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("upgrade http to websocket", sl.Err(err))
		return
	}
	remote := r.RemoteAddr
	// if the request is coming from a proxy, use the X-Forwarded-For header
	xRemote := r.Header.Get("X-Forwarded-For")
	if xRemote != "" {
		remote = xRemote
	}

	client := &Client{
		ws:           ws,
		core:         s.core,
		statusReader: s.statusReader,
		send:         make(chan []byte, 256),
		logger:       s.log.With(slog.String("remote", remote)),
		pool:         s.pool,
		id:           "",
		subscription: ChargePointEvent,
		listeners:    make(map[int]string),
		mux:          sync.Mutex{},
	}
	s.pool.register <- client

	go client.writePump()
	go client.readPump()
}
