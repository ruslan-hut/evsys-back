package internal

import (
	"evsys-back/config"
	"evsys-back/services"
	"fmt"
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
)

type Server struct {
	conf       *config.Config
	httpServer *http.Server
	apiHandler func(ac *Call) ([]byte, int)
	logger     services.LogHandler
}

func NewServer(conf *config.Config) *Server {
	server := Server{
		conf: conf,
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
	router.OPTIONS("/*path", s.options)
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

func (s *Server) options(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s.logger.Info(fmt.Sprintf("options request from %s", r.RemoteAddr))
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
