// Package oauth is the OAuth 2.1 authorization server that guards the MCP
// endpoint. Users never type credentials here: the authorization endpoint
// parks the request and sends the browser to the consent page in evsys-front,
// where the user signs in the usual way (Firebase or local account) and
// approves. The frontend reports the decision back with the user's regular API
// token, and only then is an authorization code issued.
//
// Clients register themselves (RFC 7591) and must use PKCE with S256. Access
// tokens are opaque and short-lived; refresh tokens rotate on every use. Both
// are stored as SHA-256 hashes only.
//
// Pending requests and authorization codes live in memory: they last minutes,
// and losing them on a restart only means the user clicks Connect again.
// Clients and tokens are persisted, so a deploy does not log anyone out.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// IssuerPath and McpPath are appended to the public URL.
	IssuerPath = "/api/v1/oauth"
	McpPath    = "/api/v1/mcp"

	accessTokenPrefix  = "evsys_at_"
	refreshTokenPrefix = "evsys_rt_"

	requestTTL = 10 * time.Minute
	codeTTL    = 5 * time.Minute
	// maxPending bounds the in-memory maps, which unauthenticated requests can
	// fill.
	maxPending = 1000

	authMethodNone       = "none"
	authMethodSecretPost = "client_secret_post"
	authMethodSecretBase = "client_secret_basic"

	grantAuthorizationCode = "authorization_code"
	grantRefreshToken      = "refresh_token"
)

type Repository interface {
	GetUser(ctx context.Context, username string) (*entity.User, error)
	SaveOAuthClient(ctx context.Context, client *entity.OAuthClient) error
	GetOAuthClient(ctx context.Context, clientId string) (*entity.OAuthClient, error)
	SaveOAuthToken(ctx context.Context, token *entity.OAuthToken) error
	GetOAuthToken(ctx context.Context, hash string) (*entity.OAuthToken, error)
	DeleteOAuthToken(ctx context.Context, hash string) (bool, error)
	DeleteOAuthGrant(ctx context.Context, grantId string) error
}

type Config struct {
	// PublicUrl is the origin MCP clients reach this backend at.
	PublicUrl string
	// FrontendUrl is the evsys-front origin hosting the consent page.
	FrontendUrl     string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func oauthError(status int, code, format string, args ...any) *entity.OAuthError {
	return &entity.OAuthError{Code: code, Description: fmt.Sprintf(format, args...), Status: status}
}

// authRequest is an authorization request waiting for the user's decision.
type authRequest struct {
	client        *entity.OAuthClient
	redirectUri   string
	state         string
	codeChallenge string
	resource      string
	expiresAt     time.Time
}

// authCode is an approved request waiting to be exchanged for tokens.
type authCode struct {
	clientId      string
	redirectUri   string
	codeChallenge string
	resource      string
	username      string
	expiresAt     time.Time
}

type Service struct {
	repo        Repository
	publicUrl   string
	frontendUrl string
	accessTTL   time.Duration
	refreshTTL  time.Duration
	now         func() time.Time
	log         *slog.Logger

	mu       sync.Mutex
	requests map[string]*authRequest
	codes    map[string]*authCode
}

func New(conf Config, repo Repository, log *slog.Logger) (*Service, error) {
	publicUrl := strings.TrimRight(conf.PublicUrl, "/")
	frontendUrl := strings.TrimRight(conf.FrontendUrl, "/")
	if publicUrl == "" || frontendUrl == "" {
		return nil, fmt.Errorf("public url and frontend url are required")
	}
	if conf.AccessTokenTTL <= 0 || conf.RefreshTokenTTL <= 0 {
		return nil, fmt.Errorf("token lifetimes must be positive")
	}
	return &Service{
		repo:        repo,
		publicUrl:   publicUrl,
		frontendUrl: frontendUrl,
		accessTTL:   conf.AccessTokenTTL,
		refreshTTL:  conf.RefreshTokenTTL,
		now:         time.Now,
		log:         log.With(sl.Module("impl.oauth")),
		requests:    make(map[string]*authRequest),
		codes:       make(map[string]*authCode),
	}, nil
}

// Issuer is the authorization server's identifier, which clients compare
// against the metadata they fetch.
func (s *Service) Issuer() string { return s.publicUrl + IssuerPath }

// Resource is the MCP endpoint, the audience of every token issued here.
func (s *Service) Resource() string { return s.publicUrl + McpPath }

// ResourceMetadataUrl is where the protected resource metadata is announced
// in 401 responses. It sits under /api/v1 so that a proxy forwarding only that
// prefix still serves it.
func (s *Service) ResourceMetadataUrl() string { return s.Issuer() + "/protected-resource" }

// AuthorizationServerMetadata is the RFC 8414 document.
func (s *Service) AuthorizationServerMetadata() map[string]any {
	return map[string]any{
		"issuer":                                         s.Issuer(),
		"authorization_endpoint":                         s.Issuer() + "/authorize",
		"token_endpoint":                                 s.Issuer() + "/token",
		"registration_endpoint":                          s.Issuer() + "/register",
		"revocation_endpoint":                            s.Issuer() + "/revoke",
		"scopes_supported":                               []string{entity.OAuthScope},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{grantAuthorizationCode, grantRefreshToken},
		"token_endpoint_auth_methods_supported":          []string{authMethodNone, authMethodSecretPost, authMethodSecretBase},
		"revocation_endpoint_auth_methods_supported":     []string{authMethodNone, authMethodSecretPost, authMethodSecretBase},
		"code_challenge_methods_supported":               []string{"S256"},
		"authorization_response_iss_parameter_supported": true,
	}
}

// ProtectedResourceMetadata is the RFC 9728 document for the MCP endpoint.
func (s *Service) ProtectedResourceMetadata() map[string]any {
	return map[string]any{
		"resource":                 s.Resource(),
		"authorization_servers":    []string{s.Issuer()},
		"scopes_supported":         []string{entity.OAuthScope},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "EVSys",
	}
}

// --- Dynamic client registration (RFC 7591) ---

const (
	maxRedirectUris  = 10
	maxClientNameLen = 100
)

func (s *Service) RegisterClient(ctx context.Context, md *entity.OAuthClientMetadata) (*entity.OAuthClientInformation, error) {
	if md == nil || len(md.RedirectUris) == 0 {
		return nil, oauthError(http.StatusBadRequest, "invalid_redirect_uri", "at least one redirect_uri is required")
	}
	if len(md.RedirectUris) > maxRedirectUris {
		return nil, oauthError(http.StatusBadRequest, "invalid_redirect_uri", "at most %d redirect_uris are accepted", maxRedirectUris)
	}
	for _, uri := range md.RedirectUris {
		if err := validateRedirectUri(uri); err != nil {
			return nil, oauthError(http.StatusBadRequest, "invalid_redirect_uri", "%s: %v", uri, err)
		}
	}
	for _, gt := range md.GrantTypes {
		if gt != grantAuthorizationCode && gt != grantRefreshToken {
			return nil, oauthError(http.StatusBadRequest, "invalid_client_metadata", "unsupported grant type %q", gt)
		}
	}
	for _, rt := range md.ResponseTypes {
		if rt != "code" {
			return nil, oauthError(http.StatusBadRequest, "invalid_client_metadata", "unsupported response type %q", rt)
		}
	}
	method := md.TokenEndpointAuthMethod
	switch method {
	case "":
		// RFC 7591 defaults to client_secret_basic, but MCP clients that omit
		// the field expect a public client; PKCE protects either kind.
		method = authMethodNone
	case authMethodNone, authMethodSecretPost, authMethodSecretBase:
	default:
		return nil, oauthError(http.StatusBadRequest, "invalid_client_metadata", "unsupported token_endpoint_auth_method %q", method)
	}
	name := strings.TrimSpace(md.ClientName)
	if runes := []rune(name); len(runes) > maxClientNameLen {
		name = string(runes[:maxClientNameLen])
	}
	if name == "" {
		name = "MCP client"
	}

	now := s.now()
	client := &entity.OAuthClient{
		ClientId:                "mcp_" + randomToken(16),
		ClientName:              name,
		RedirectUris:            md.RedirectUris,
		TokenEndpointAuthMethod: method,
		CreatedAt:               now,
	}
	info := &entity.OAuthClientInformation{
		ClientId:                client.ClientId,
		ClientIdIssuedAt:        now.Unix(),
		ClientName:              client.ClientName,
		RedirectUris:            client.RedirectUris,
		GrantTypes:              []string{grantAuthorizationCode, grantRefreshToken},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: method,
		Scope:                   entity.OAuthScope,
	}
	if method != authMethodNone {
		secret := randomToken(32)
		client.SecretHash = hashToken(secret)
		info.ClientSecret = secret
		never := int64(0)
		info.ClientSecretExpiresAt = &never
	}
	if err := s.repo.SaveOAuthClient(ctx, client); err != nil {
		return nil, fmt.Errorf("save client: %w", err)
	}
	s.log.With(
		slog.String("client_id", client.ClientId),
		slog.String("client_name", client.ClientName),
		slog.String("auth_method", method),
		slog.Any("redirect_uris", client.RedirectUris),
	).Info("oauth client registered")
	return info, nil
}

// --- Authorization endpoint ---

// Authorize validates an authorization request and parks it for the consent
// page. On success it returns the consent page URL. When the client and
// redirect URI check out but the request is otherwise wrong, it returns the
// redirect URI carrying the error, as RFC 6749 requires. An error means the
// redirect target itself cannot be trusted and the request must be answered
// directly.
func (s *Service) Authorize(ctx context.Context, req entity.OAuthAuthorizeRequest) (string, error) {
	if req.ClientId == "" {
		return "", oauthError(http.StatusBadRequest, "invalid_request", "client_id is required")
	}
	client, err := s.repo.GetOAuthClient(ctx, req.ClientId)
	if err != nil {
		return "", fmt.Errorf("get client: %w", err)
	}
	if client == nil {
		return "", oauthError(http.StatusBadRequest, "invalid_client", "unknown client_id")
	}
	redirectUri := req.RedirectUri
	if redirectUri == "" {
		if len(client.RedirectUris) != 1 {
			return "", oauthError(http.StatusBadRequest, "invalid_request", "redirect_uri is required")
		}
		redirectUri = client.RedirectUris[0]
	} else if !matchRedirectUri(client.RedirectUris, redirectUri) {
		return "", oauthError(http.StatusBadRequest, "invalid_request", "redirect_uri is not registered for this client")
	}

	fail := func(code, description string) (string, error) {
		return s.errorRedirect(redirectUri, req.State, code, description), nil
	}
	if req.ResponseType != "code" {
		return fail("unsupported_response_type", "only response_type=code is supported")
	}
	if req.CodeChallenge == "" || req.CodeChallengeMethod != "S256" {
		return fail("invalid_request", "PKCE with code_challenge_method=S256 is required")
	}
	if req.Resource != "" && !s.isResource(req.Resource) {
		return fail("invalid_target", "unknown resource")
	}

	id := randomToken(32)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	if len(s.requests) >= maxPending {
		return fail("temporarily_unavailable", "too many pending authorization requests")
	}
	s.requests[id] = &authRequest{
		client:        client,
		redirectUri:   redirectUri,
		state:         req.State,
		codeChallenge: req.CodeChallenge,
		resource:      s.Resource(),
		expiresAt:     s.now().Add(requestTTL),
	}
	return s.frontendUrl + "/oauth/authorize?request=" + id, nil
}

// --- Consent, called by evsys-front on behalf of a signed-in user ---

// Consent describes a pending request to the consent page.
func (s *Service) Consent(_ context.Context, requestId string, user *entity.User) (*entity.OAuthConsent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestId]
	if !ok || s.now().After(req.expiresAt) {
		return nil, fmt.Errorf("authorization request %w or expired", entity.ErrNotFound)
	}
	return &entity.OAuthConsent{
		RequestId:    requestId,
		ClientId:     req.client.ClientId,
		ClientName:   req.client.ClientName,
		RedirectUri:  req.redirectUri,
		RedirectHost: redirectHost(req.redirectUri),
		KnownClient:  isKnownRedirect(req.redirectUri),
		Scope:        entity.OAuthScope,
		Resource:     req.resource,
		ExpiresAt:    req.expiresAt,
		Username:     user.Username,
		CanApprove:   user.IsPowerUser(),
	}, nil
}

// Approve issues an authorization code for the user and returns the client
// redirect carrying it. The request is consumed.
func (s *Service) Approve(_ context.Context, requestId string, user *entity.User) (*entity.OAuthRedirect, error) {
	if !user.IsPowerUser() {
		return nil, entity.ErrForbidden
	}
	req, err := s.takeRequest(requestId)
	if err != nil {
		return nil, err
	}
	code := randomToken(32)
	s.mu.Lock()
	s.sweepLocked()
	s.codes[hashToken(code)] = &authCode{
		clientId:      req.client.ClientId,
		redirectUri:   req.redirectUri,
		codeChallenge: req.codeChallenge,
		resource:      req.resource,
		username:      user.Username,
		expiresAt:     s.now().Add(codeTTL),
	}
	s.mu.Unlock()

	s.log.With(
		slog.String("client_id", req.client.ClientId),
		slog.String("client_name", req.client.ClientName),
		slog.String("user", user.Username),
	).Info("oauth access approved")
	return &entity.OAuthRedirect{
		RedirectUrl: appendQuery(req.redirectUri, map[string]string{"code": code, "state": req.state, "iss": s.Issuer()}),
	}, nil
}

// Deny consumes the request and returns the client redirect carrying
// access_denied.
func (s *Service) Deny(_ context.Context, requestId string, user *entity.User) (*entity.OAuthRedirect, error) {
	req, err := s.takeRequest(requestId)
	if err != nil {
		return nil, err
	}
	s.log.With(
		slog.String("client_id", req.client.ClientId),
		slog.String("user", user.Username),
	).Info("oauth access denied")
	return &entity.OAuthRedirect{
		RedirectUrl: s.errorRedirect(req.redirectUri, req.state, "access_denied", "the user denied access"),
	}, nil
}

func (s *Service) takeRequest(requestId string) (*authRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req, ok := s.requests[requestId]
	if ok {
		delete(s.requests, requestId)
	}
	if !ok || s.now().After(req.expiresAt) {
		return nil, fmt.Errorf("authorization request %w or expired", entity.ErrNotFound)
	}
	return req, nil
}

// sweepLocked drops expired requests and codes. Callers hold s.mu.
func (s *Service) sweepLocked() {
	now := s.now()
	for id, req := range s.requests {
		if now.After(req.expiresAt) {
			delete(s.requests, id)
		}
	}
	for hash, code := range s.codes {
		if now.After(code.expiresAt) {
			delete(s.codes, hash)
		}
	}
}

// --- Token endpoint ---

func (s *Service) Token(ctx context.Context, req entity.OAuthTokenRequest) (*entity.OAuthTokenResponse, error) {
	client, err := s.authenticateClient(ctx, req.ClientId, req.ClientSecret)
	if err != nil {
		return nil, err
	}
	if req.Resource != "" && !s.isResource(req.Resource) {
		return nil, oauthError(http.StatusBadRequest, "invalid_target", "unknown resource")
	}
	switch req.GrantType {
	case grantAuthorizationCode:
		return s.exchangeCode(ctx, client, req)
	case grantRefreshToken:
		return s.refresh(ctx, client, req)
	default:
		return nil, oauthError(http.StatusBadRequest, "unsupported_grant_type", "grant_type %q is not supported", req.GrantType)
	}
}

func (s *Service) exchangeCode(ctx context.Context, client *entity.OAuthClient, req entity.OAuthTokenRequest) (*entity.OAuthTokenResponse, error) {
	if req.Code == "" || req.CodeVerifier == "" {
		return nil, oauthError(http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
	}
	// The code is single use: it is removed before anything is checked, so a
	// failed attempt burns it too.
	s.mu.Lock()
	hash := hashToken(req.Code)
	code, ok := s.codes[hash]
	delete(s.codes, hash)
	s.mu.Unlock()

	if !ok || s.now().After(code.expiresAt) || code.clientId != client.ClientId {
		return nil, oauthError(http.StatusBadRequest, "invalid_grant", "authorization code is invalid or expired")
	}
	if req.RedirectUri != "" && req.RedirectUri != code.redirectUri {
		return nil, oauthError(http.StatusBadRequest, "invalid_grant", "redirect_uri does not match the authorization request")
	}
	if !verifyPKCE(req.CodeVerifier, code.codeChallenge) {
		return nil, oauthError(http.StatusBadRequest, "invalid_grant", "code_verifier does not match the code challenge")
	}
	user, err := s.activeUser(ctx, code.username)
	if err != nil {
		return nil, oauthError(http.StatusBadRequest, "invalid_grant", "%v", err)
	}
	resp, err := s.issueTokens(ctx, client.ClientId, user.Username, code.resource, randomToken(16))
	if err != nil {
		return nil, err
	}
	s.log.With(
		slog.String("client_id", client.ClientId),
		slog.String("user", user.Username),
	).Info("oauth tokens issued")
	return resp, nil
}

func (s *Service) refresh(ctx context.Context, client *entity.OAuthClient, req entity.OAuthTokenRequest) (*entity.OAuthTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, oauthError(http.StatusBadRequest, "invalid_request", "refresh_token is required")
	}
	invalid := oauthError(http.StatusBadRequest, "invalid_grant", "refresh token is invalid or expired")
	hash := hashToken(req.RefreshToken)
	stored, err := s.repo.GetOAuthToken(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	if stored == nil || stored.Kind != entity.OAuthRefreshToken || stored.ClientId != client.ClientId {
		return nil, invalid
	}
	// Deleting claims the token: two requests racing with the same refresh
	// token cannot both get a new pair.
	claimed, err := s.repo.DeleteOAuthToken(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("delete token: %w", err)
	}
	if !claimed || s.now().After(stored.ExpiresAt) {
		return nil, invalid
	}
	user, err := s.activeUser(ctx, stored.Username)
	if err != nil {
		_ = s.repo.DeleteOAuthGrant(ctx, stored.GrantId)
		return nil, oauthError(http.StatusBadRequest, "invalid_grant", "%v", err)
	}
	return s.issueTokens(ctx, client.ClientId, user.Username, stored.Resource, stored.GrantId)
}

func (s *Service) issueTokens(ctx context.Context, clientId, username, resource, grantId string) (*entity.OAuthTokenResponse, error) {
	now := s.now()
	access := accessTokenPrefix + randomToken(32)
	refresh := refreshTokenPrefix + randomToken(32)
	tokens := []*entity.OAuthToken{
		{Hash: hashToken(access), Kind: entity.OAuthAccessToken, ExpiresAt: now.Add(s.accessTTL)},
		{Hash: hashToken(refresh), Kind: entity.OAuthRefreshToken, ExpiresAt: now.Add(s.refreshTTL)},
	}
	for _, t := range tokens {
		t.GrantId = grantId
		t.ClientId = clientId
		t.Username = username
		t.Scope = entity.OAuthScope
		t.Resource = resource
		t.CreatedAt = now
		if err := s.repo.SaveOAuthToken(ctx, t); err != nil {
			return nil, fmt.Errorf("save token: %w", err)
		}
	}
	return &entity.OAuthTokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        entity.OAuthScope,
	}, nil
}

// authenticateClient checks the client id and, for confidential clients, the
// secret.
func (s *Service) authenticateClient(ctx context.Context, clientId, secret string) (*entity.OAuthClient, error) {
	invalid := oauthError(http.StatusUnauthorized, "invalid_client", "client authentication failed")
	if clientId == "" {
		return nil, invalid
	}
	client, err := s.repo.GetOAuthClient(ctx, clientId)
	if err != nil {
		return nil, fmt.Errorf("get client: %w", err)
	}
	if client == nil {
		return nil, invalid
	}
	if client.SecretHash != "" {
		if secret == "" || subtle.ConstantTimeCompare([]byte(hashToken(secret)), []byte(client.SecretHash)) != 1 {
			return nil, invalid
		}
	}
	return client, nil
}

// --- Revocation (RFC 7009) ---

// Revoke ends the grant a token belongs to, so that revoking either the access
// or the refresh token disconnects the client. Unknown tokens are not an
// error: the answer must not reveal whether a token existed.
func (s *Service) Revoke(ctx context.Context, token, clientId, secret string) error {
	client, err := s.authenticateClient(ctx, clientId, secret)
	if err != nil {
		return err
	}
	stored, err := s.repo.GetOAuthToken(ctx, hashToken(token))
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	if stored == nil || stored.ClientId != client.ClientId {
		return nil
	}
	if err = s.repo.DeleteOAuthGrant(ctx, stored.GrantId); err != nil {
		return fmt.Errorf("delete grant: %w", err)
	}
	s.log.With(
		slog.String("client_id", client.ClientId),
		slog.String("user", stored.Username),
	).Info("oauth grant revoked")
	return nil
}

// --- Resource server side ---

// VerifyAccessToken resolves a bearer token presented to the MCP endpoint to
// the user it acts for. The user is read fresh on every call, so demoting an
// admin cuts off their MCP access at once rather than at token expiry.
func (s *Service) VerifyAccessToken(ctx context.Context, token string) (*entity.User, *entity.OAuthToken, error) {
	if !strings.HasPrefix(token, accessTokenPrefix) {
		return nil, nil, entity.ErrInvalidToken
	}
	stored, err := s.repo.GetOAuthToken(ctx, hashToken(token))
	if err != nil {
		return nil, nil, fmt.Errorf("get token: %w", err)
	}
	if stored == nil || stored.Kind != entity.OAuthAccessToken || s.now().After(stored.ExpiresAt) || stored.Resource != s.Resource() {
		return nil, nil, entity.ErrInvalidToken
	}
	user, err := s.activeUser(ctx, stored.Username)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", entity.ErrInvalidToken, err)
	}
	return user, stored, nil
}

// activeUser loads a user and checks they may still use MCP.
func (s *Service) activeUser(ctx context.Context, username string) (*entity.User, error) {
	user, err := s.repo.GetUser(ctx, username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user %s not found", username)
	}
	if !user.IsPowerUser() {
		return nil, fmt.Errorf("user %s: %w", username, entity.ErrForbidden)
	}
	clean := *user
	clean.Password = ""
	clean.Token = ""
	return &clean, nil
}

func (s *Service) isResource(resource string) bool {
	return strings.TrimRight(resource, "/") == s.Resource()
}

func (s *Service) errorRedirect(redirectUri, state, code, description string) string {
	return appendQuery(redirectUri, map[string]string{
		"error":             code,
		"error_description": description,
		"state":             state,
		"iss":               s.Issuer(),
	})
}

// --- helpers ---

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail on supported platforms; carrying on with
		// a predictable token would be worse than stopping.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// verifyPKCE checks an S256 code verifier (RFC 7636 section 4.6).
func verifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
