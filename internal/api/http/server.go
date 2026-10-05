package http

import (
	"context"
	"evsys-back/config"
	centralsystem "evsys-back/internal/api/handlers/central-system"
	"evsys-back/internal/api/handlers/helper"
	"evsys-back/internal/api/handlers/locations"
	"evsys-back/internal/api/handlers/mail"
	oauthapi "evsys-back/internal/api/handlers/oauth"
	"evsys-back/internal/api/handlers/payments"
	"evsys-back/internal/api/handlers/report"
	"evsys-back/internal/api/handlers/transactions"
	"evsys-back/internal/api/handlers/users"
	"evsys-back/internal/api/handlers/usertags"
	"evsys-back/internal/api/handlers/webhooks"
	"evsys-back/internal/api/middleware/apikey"
	"evsys-back/internal/api/middleware/authenticate"
	"evsys-back/internal/api/middleware/authorize"
	"evsys-back/internal/api/middleware/timeout"
	"evsys-back/internal/api/websocket"
	"evsys-back/internal/lib/sl"
	"evsys-back/internal/mcpserver"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	ws "github.com/gorilla/websocket"
)

type Server struct {
	conf            *config.Config
	httpServer      *http.Server
	core            Core
	statusReader    websocket.StatusReader
	log             *slog.Logger
	upgrader        ws.Upgrader
	pool            *websocket.Pool
	broadcaster     *websocket.Broadcaster
	cancelBroadcast context.CancelFunc
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
	payments.Preauthorizations
	payments.DirectPayments
	payments.RetryQueue
	report.Reports
	mail.Handler
	webhooks.Handler
	mcpserver.Core

	websocket.Core
}

// OAuth is the authorization server guarding the MCP endpoint.
type OAuth interface {
	oauthapi.Service
	mcpserver.TokenVerifier
	ResourceMetadataUrl() string
}

const (
	// apiTimeout bounds REST requests, in seconds.
	apiTimeout = 5
	// defaultMcpTimeout bounds an MCP request when the config leaves it unset.
	defaultMcpTimeout = 60
)

// NewServer builds the HTTP server. oauth may be nil, which leaves the MCP
// endpoint and the OAuth routes out.
func NewServer(conf *config.Config, log *slog.Logger, core Core, oauth OAuth) *Server {

	server := Server{
		conf: conf,
		core: core,
		log:  log.With(sl.Module("api.server")),
		upgrader: ws.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Use(helper.Options())

	// The MCP endpoint has its own timeout: a report over a long period takes
	// longer than the REST budget, and the tools read only.
	if oauth != nil {
		mcpTimeout := time.Duration(conf.Mcp.RequestTimeout)
		if mcpTimeout <= 0 {
			mcpTimeout = defaultMcpTimeout
		}
		router.With(timeout.Timeout(mcpTimeout)).Handle("/api/v1/mcp",
			mcpserver.NewHandler(log, core, oauth, oauth.ResourceMetadataUrl()))
	}

	router.Group(func(router chi.Router) {
		router.Use(timeout.Timeout(apiTimeout))

		server.routes(router, log, core, oauth)
	})

	server.httpServer = &http.Server{
		Handler: router,
	}

	return &server
}

func (s *Server) routes(router chi.Router, log *slog.Logger, core Core, oauth OAuth) {
	// websocket connection
	router.Route("/", func(r chi.Router) {
		r.Get("/ws", s.handleWs)

		// OAuth discovery at the well-known locations (RFC 8414, RFC 9728).
		// Clients reach them only if the proxy forwards /.well-known to this
		// service; the copies under /api/v1/oauth below work without that.
		if oauth != nil {
			r.Get("/.well-known/oauth-protected-resource", oauthapi.ProtectedResourceMetadata(oauth))
			r.Get("/.well-known/oauth-protected-resource/*", oauthapi.ProtectedResourceMetadata(oauth))
			r.Get("/.well-known/oauth-authorization-server", oauthapi.AuthorizationServerMetadata(oauth))
			r.Get("/.well-known/oauth-authorization-server/*", oauthapi.AuthorizationServerMetadata(oauth))
			r.Get("/.well-known/openid-configuration/*", oauthapi.AuthorizationServerMetadata(oauth))
		}
	})

	router.Route("/api/v1", func(r chi.Router) {
		// requests with authorization token
		r.Group(func(r chi.Router) {
			r.Use(authenticate.New(log, core))

			// consent page of evsys-front, deciding on an MCP client
			if oauth != nil {
				r.Get("/oauth/requests/{id}", oauthapi.ConsentInfo(log, oauth))
				r.Post("/oauth/requests/{id}/approve", oauthapi.ConsentApprove(log, oauth))
				r.Post("/oauth/requests/{id}/deny", oauthapi.ConsentDeny(log, oauth))
			}

			r.Get("/locations", locations.ListLocations(log, core))
			r.Get("/chp", locations.ListChargePoints(log, core))
			r.Get("/chp/{search}", locations.ListChargePoints(log, core))
			r.Get("/point/{id}", locations.ChargePointRead(log, core))
			r.Post("/point/{id}", locations.ChargePointSave(log, core))

			r.Get("/users/info/{name}", users.Info(log, core))
			r.Get("/users/list", users.List(log, core))

			// power user only routes
			r.Group(func(r chi.Router) {
				r.Use(authorize.RequirePowerUser(log))

				r.Post("/users/create", users.Create(log, core))
				r.Put("/users/update/{username}", users.Update(log, core))
				r.Delete("/users/delete/{username}", users.Delete(log, core))

				r.Get("/user-tags/list", usertags.List(log, core))
				r.Get("/user-tags/info/{idTag}", usertags.Info(log, core))
				r.Post("/user-tags/create", usertags.Create(log, core))
				r.Put("/user-tags/update/{idTag}", usertags.Update(log, core))
				r.Delete("/user-tags/delete/{idTag}", usertags.Delete(log, core))

				r.Get("/mail/subscriptions", mail.List(log, core))
				r.Post("/mail/subscriptions", mail.Create(log, core))
				r.Put("/mail/subscriptions/{id}", mail.Update(log, core))
				r.Delete("/mail/subscriptions/{id}", mail.Delete(log, core))
				r.Post("/mail/subscriptions/{id}/send-now", mail.SendNow(log, core))
				r.Post("/mail/test", mail.Test(log, core))

				r.Get("/payment/retries", payments.RetryQueueList(log, core))
				r.Post("/payment/retries/{transactionId}/force", payments.RetryQueueForce(log, core))

				r.Get("/webhooks/subscribers", webhooks.List(log, core))
				r.Post("/webhooks/subscribers", webhooks.Create(log, core))
				r.Put("/webhooks/subscribers/{id}", webhooks.Update(log, core))
				r.Delete("/webhooks/subscribers/{id}", webhooks.Delete(log, core))
				r.Get("/webhooks/health", webhooks.Health(log, core))
				r.Get("/webhooks/failures", webhooks.Failures(log, core))
			})

			r.Post("/csc", centralsystem.Command(log, core))

			r.Get("/transactions/active", transactions.ListActive(log, core))
			r.Get("/transactions/list", transactions.List(log, core))
			r.Get("/transactions/recent", transactions.RecentUserChargePoints(log, core))
			r.Get("/transactions/list/{period}", transactions.List(log, core))
			r.Get("/transactions/info/{id}", transactions.Get(log, core))
			r.Post("/transactions/info/{id}/email", transactions.SendMail(log, core))
			r.Get("/transactions/info/{id}/receipt", transactions.Receipt(log, core))
			//router.Get("/transactions/bill", s.transactionBill)

			r.Get("/payment/methods", payments.List(log, core))
			r.Post("/payment/save", payments.Save(log, core))
			r.Post("/payment/update", payments.Update(log, core))
			r.Post("/payment/delete", payments.Delete(log, core))
			r.Post("/payment/order", payments.Order(log, core))

			r.Get("/report/month", report.MonthlyStatistics(log, core))
			r.Get("/report/user", report.UsersStatistics(log, core))
			r.Get("/report/charger", report.ChargerStatistics(log, core))
			r.Get("/report/export", report.ExportStatistics(log, core))
			r.Get("/report/power", report.PowerStatistics(log, core))
			r.Get("/report/uptime", report.StationUptimeStatistics(log, core))
			r.Get("/report/status", report.StationStatusStatistics(log, core))
			r.Get("/report/concurrency", report.SiteConcurrencyStatistics(log, core))

			r.Get("/log/{name}", helper.Log(log, core))
		})

		// service-to-service payment endpoints (API key auth)
		if s.conf.Redsys.ApiKey != "" {
			r.Group(func(r chi.Router) {
				r.Use(apikey.New(log, s.conf.Redsys.ApiKey))

				r.Get("/payment/pay/{transactionId}", payments.Pay(log, core))
				r.Get("/payment/return/{transactionId}", payments.Return(log, core))
				r.Post("/payment/return/order/{orderId}", payments.ReturnByOrder(log, core))
			})
		}

		// requests without authorization token
		r.Group(func(r chi.Router) {
			r.Get("/config/{name}", helper.Config(log, core))
			r.Post("/users/authenticate", users.Authenticate(log, core))
			r.Post("/users/register", users.Register(log, core))
			r.Post("/payment/notify", payments.Notify(log, core))

			// OAuth authorization server for MCP clients
			if oauth != nil {
				r.Get("/oauth/protected-resource", oauthapi.ProtectedResourceMetadata(oauth))
				r.Get("/oauth/.well-known/oauth-authorization-server", oauthapi.AuthorizationServerMetadata(oauth))
				r.Get("/oauth/.well-known/openid-configuration", oauthapi.AuthorizationServerMetadata(oauth))
				r.Post("/oauth/register", oauthapi.Register(log, oauth))
				r.Get("/oauth/authorize", oauthapi.Authorize(log, oauth))
				r.Post("/oauth/token", oauthapi.Token(log, oauth))
				r.Post("/oauth/revoke", oauthapi.Revoke(log, oauth))
			}
		})
	})
}

func (s *Server) SetStatusReader(statusReader websocket.StatusReader) {
	s.statusReader = statusReader
}

func (s *Server) Start() error {
	if s.conf == nil {
		return fmt.Errorf("configuration not loaded")
	}
	if s.core == nil {
		return fmt.Errorf("core handler not set")
	}

	s.pool = websocket.NewPool(s.log)
	go s.pool.Start()

	// start listening for log updates, if update received, send it to all subscribed clients
	s.broadcaster = websocket.NewBroadcaster(s.pool, s.statusReader, s.log)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelBroadcast = cancel
	go s.broadcaster.Start(ctx)

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
	if s.cancelBroadcast != nil {
		s.cancelBroadcast()
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleWs(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
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

	client := websocket.NewClient(
		conn,
		s.pool,
		s.core,
		s.statusReader,
		s.log.With(slog.String("remote", remote)),
	)
	client.Start()
}
