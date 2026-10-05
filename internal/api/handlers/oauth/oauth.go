// Package oauth serves the OAuth 2.1 endpoints that let MCP clients connect:
// discovery metadata, client registration, authorization, token and
// revocation, plus the consent calls evsys-front makes for a signed-in user.
//
// Protocol endpoints answer in the RFC 6749 error format, not the
// response.Error envelope of the rest of the API, because OAuth client
// libraries parse them.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

const maxBodyBytes = 64 << 10

type Service interface {
	AuthorizationServerMetadata() map[string]any
	ProtectedResourceMetadata() map[string]any
	RegisterClient(ctx context.Context, md *entity.OAuthClientMetadata) (*entity.OAuthClientInformation, error)
	Authorize(ctx context.Context, req entity.OAuthAuthorizeRequest) (string, error)
	Token(ctx context.Context, req entity.OAuthTokenRequest) (*entity.OAuthTokenResponse, error)
	Revoke(ctx context.Context, token, clientId, secret string) error
	Consent(ctx context.Context, requestId string, user *entity.User) (*entity.OAuthConsent, error)
	Approve(ctx context.Context, requestId string, user *entity.User) (*entity.OAuthRedirect, error)
	Deny(ctx context.Context, requestId string, user *entity.User) (*entity.OAuthRedirect, error)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError answers with an OAuth error, or a server error for anything else.
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	var oe *entity.OAuthError
	if errors.As(err, &oe) {
		log.With(sl.Err(err)).Warn("oauth request rejected")
		if oe.Status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="evsys"`)
		}
		writeJSON(w, oe.Status, oe)
		return
	}
	log.With(sl.Err(err)).Error("oauth request failed")
	writeJSON(w, http.StatusInternalServerError, &entity.OAuthError{Code: "server_error"})
}

// metadata serves a public discovery document. Any origin may read it: it is
// what browser-based MCP clients start from.
func metadata(document func() map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(document())
	}
}

// AuthorizationServerMetadata serves RFC 8414 metadata.
func AuthorizationServerMetadata(svc Service) http.HandlerFunc {
	return metadata(svc.AuthorizationServerMetadata)
}

// ProtectedResourceMetadata serves RFC 9728 metadata for the MCP endpoint.
func ProtectedResourceMetadata(svc Service) http.HandlerFunc {
	return metadata(svc.ProtectedResourceMetadata)
}

// Register handles dynamic client registration (RFC 7591).
func Register(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := web.Log(r.Context(), logger, "handlers.oauth", slog.String("endpoint", "register"))
		var md entity.OAuthClientMetadata
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&md); err != nil {
			writeError(w, log, &entity.OAuthError{Code: "invalid_client_metadata", Description: "request body is not valid JSON", Status: http.StatusBadRequest})
			return
		}
		info, err := svc.RegisterClient(r.Context(), &md)
		if err != nil {
			writeError(w, log, err)
			return
		}
		writeJSON(w, http.StatusCreated, info)
	}
}

// Authorize starts an authorization: it sends the browser to the consent page
// in evsys-front, or back to the client with an error. A request whose client
// or redirect URI does not check out gets a plain error page, since there is
// nowhere safe to redirect it.
func Authorize(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := web.Log(r.Context(), logger, "handlers.oauth", slog.String("endpoint", "authorize"))
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		q := r.Form
		target, err := svc.Authorize(r.Context(), entity.OAuthAuthorizeRequest{
			ResponseType:        q.Get("response_type"),
			ClientId:            q.Get("client_id"),
			RedirectUri:         q.Get("redirect_uri"),
			Scope:               q.Get("scope"),
			State:               q.Get("state"),
			CodeChallenge:       q.Get("code_challenge"),
			CodeChallengeMethod: q.Get("code_challenge_method"),
			Resource:            q.Get("resource"),
		})
		if err != nil {
			var oe *entity.OAuthError
			if errors.As(err, &oe) {
				log.With(sl.Err(err), slog.String("client_id", q.Get("client_id"))).Warn("authorization request rejected")
				http.Error(w, "Authorization request rejected: "+oe.Error(), oe.Status)
				return
			}
			log.With(sl.Err(err)).Error("authorization request failed")
			http.Error(w, "Authorization failed, try again later", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// clientCredentials reads the client id and secret from HTTP Basic
// authentication (RFC 6749 section 2.3.1, form-encoded) or from the form.
func clientCredentials(r *http.Request) (string, string) {
	if id, secret, ok := r.BasicAuth(); ok {
		if decoded, err := url.QueryUnescape(id); err == nil {
			id = decoded
		}
		if decoded, err := url.QueryUnescape(secret); err == nil {
			secret = decoded
		}
		return id, secret
	}
	return r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
}

// Token handles the token endpoint: authorization code and refresh token
// grants.
func Token(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := web.Log(r.Context(), logger, "handlers.oauth", slog.String("endpoint", "token"))
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := r.ParseForm(); err != nil {
			writeError(w, log, &entity.OAuthError{Code: "invalid_request", Description: "cannot parse form", Status: http.StatusBadRequest})
			return
		}
		clientId, secret := clientCredentials(r)
		f := r.PostForm
		resp, err := svc.Token(r.Context(), entity.OAuthTokenRequest{
			GrantType:    f.Get("grant_type"),
			Code:         f.Get("code"),
			RedirectUri:  f.Get("redirect_uri"),
			CodeVerifier: f.Get("code_verifier"),
			RefreshToken: f.Get("refresh_token"),
			ClientId:     clientId,
			ClientSecret: secret,
			Resource:     f.Get("resource"),
		})
		if err != nil {
			writeError(w, log.With(slog.String("client_id", clientId), slog.String("grant_type", f.Get("grant_type"))), err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// Revoke handles token revocation (RFC 7009).
func Revoke(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := web.Log(r.Context(), logger, "handlers.oauth", slog.String("endpoint", "revoke"))
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		if err := r.ParseForm(); err != nil {
			writeError(w, log, &entity.OAuthError{Code: "invalid_request", Description: "cannot parse form", Status: http.StatusBadRequest})
			return
		}
		clientId, secret := clientCredentials(r)
		if err := svc.Revoke(r.Context(), r.PostForm.Get("token"), clientId, secret); err != nil {
			writeError(w, log, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// --- consent, called by evsys-front with the user's API token ---

func consentLog(logger *slog.Logger, r *http.Request, user *entity.User) *slog.Logger {
	return web.Log(r.Context(), logger, "handlers.oauth",
		slog.String("author", user.Username),
		slog.String("role", user.Role),
	)
}

func consentStatus(err error) int {
	switch {
	case errors.Is(err, entity.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, entity.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}

// ConsentInfo describes a pending authorization request to the consent page.
func ConsentInfo(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := consentLog(logger, r, user)
		data, err := svc.Consent(r.Context(), chi.URLParam(r, "id"), user)
		if err != nil {
			web.Fail(w, r, log, consentStatus(err), "Authorization request not available", err)
			return
		}
		web.OK(w, r, log, "oauth consent requested", data)
	}
}

// ConsentApprove grants the client access as the signed-in user.
func ConsentApprove(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := consentLog(logger, r, user)
		data, err := svc.Approve(r.Context(), chi.URLParam(r, "id"), user)
		if err != nil {
			web.Fail(w, r, log, consentStatus(err), "Failed to approve access", err)
			return
		}
		web.OK(w, r, log, "oauth access approved", data)
	}
}

// ConsentDeny refuses the client access.
func ConsentDeny(logger *slog.Logger, svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := cont.GetUser(r.Context())
		log := consentLog(logger, r, user)
		data, err := svc.Deny(r.Context(), chi.URLParam(r, "id"), user)
		if err != nil {
			web.Fail(w, r, log, consentStatus(err), "Failed to deny access", err)
			return
		}
		web.OK(w, r, log, "oauth access denied", data)
	}
}
