package http

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"evsys-back/config"
	"evsys-back/entity"
	"evsys-back/impl/authenticator"
	"evsys-back/impl/core"
	databasemock "evsys-back/impl/database-mock"
	"evsys-back/impl/oauth"
	"evsys-back/impl/reports"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	adminApiToken = "adm00000000000000000000000000001"
	userApiToken  = "usr00000000000000000000000000001"
	publicUrl     = "https://evsys.test"
	frontendUrl   = "https://front.test"
	verifier      = "a-long-enough-pkce-code-verifier-for-the-test-0123456789"
)

func newMcpTestServer(t *testing.T, withOAuth bool) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := databasemock.NewMockDB()
	db.SeedUser(&entity.User{Username: "boss", Role: "admin", AccessLevel: 10, Token: adminApiToken})
	db.SeedUser(&entity.User{Username: "driver", Role: "user", AccessLevel: 1, Token: userApiToken})

	c := core.New(log, db)
	c.SetAuth(authenticator.New(log, db))
	c.SetReports(reports.New(db, log))

	conf := &config.Config{}
	conf.Mcp.RequestTimeout = 60

	var oauthService OAuth
	if withOAuth {
		svc, err := oauth.New(oauth.Config{
			PublicUrl:       publicUrl,
			FrontendUrl:     frontendUrl,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		}, db, log)
		require.NoError(t, err)
		oauthService = svc
	}
	srv := httptest.NewServer(NewServer(conf, log, c, oauthService).httpServer.Handler)
	t.Cleanup(srv.Close)
	return srv
}

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func do(t *testing.T, method, url, token, contentType, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := noRedirect.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, data
}

func decode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out), string(data))
	return out
}

type bearer struct{ token string }

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestMcpOAuthFlow(t *testing.T) {
	srv := newMcpTestServer(t, true)

	// discovery, both at the well-known location and under /api/v1
	for _, path := range []string{"/.well-known/oauth-protected-resource/api/v1/mcp", "/api/v1/oauth/protected-resource"} {
		resp, body := do(t, http.MethodGet, srv.URL+path, "", "", "")
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		prm := decode(t, body)
		assert.Equal(t, publicUrl+"/api/v1/mcp", prm["resource"])
		assert.Equal(t, []any{publicUrl + "/api/v1/oauth"}, prm["authorization_servers"])
	}
	for _, path := range []string{"/.well-known/oauth-authorization-server/api/v1/oauth", "/api/v1/oauth/.well-known/openid-configuration"} {
		resp, body := do(t, http.MethodGet, srv.URL+path, "", "", "")
		require.Equal(t, http.StatusOK, resp.StatusCode, path)
		assert.Equal(t, publicUrl+"/api/v1/oauth/token", decode(t, body)["token_endpoint"])
	}

	// the MCP endpoint points unauthenticated clients at the metadata
	resp, _ := do(t, http.MethodPost, srv.URL+"/api/v1/mcp", "", "application/json", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `resource_metadata="`+publicUrl+`/api/v1/oauth/protected-resource"`)

	// registration
	redirect := "http://localhost:45678/callback"
	resp, body := do(t, http.MethodPost, srv.URL+"/api/v1/oauth/register", "", "application/json",
		`{"client_name":"Claude Code","redirect_uris":["`+redirect+`"],"token_endpoint_auth_method":"none"}`)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(body))
	clientId := decode(t, body)["client_id"].(string)

	// authorization sends the browser to the consent page
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientId},
		"redirect_uri":          {"http://localhost:51515/callback"}, // another port: loopback matches any
		"state":                 {"st"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"resource":              {publicUrl + "/api/v1/mcp"},
	}
	resp, _ = do(t, http.MethodGet, srv.URL+"/api/v1/oauth/authorize?"+q.Encode(), "", "", "")
	require.Equal(t, http.StatusFound, resp.StatusCode)
	consentUrl, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "front.test", consentUrl.Host)
	assert.Equal(t, "/oauth/authorize", consentUrl.Path)
	requestId := consentUrl.Query().Get("request")

	// the consent calls need the user's API token
	resp, _ = do(t, http.MethodGet, srv.URL+"/api/v1/oauth/requests/"+requestId, "", "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, body = do(t, http.MethodGet, srv.URL+"/api/v1/oauth/requests/"+requestId, userApiToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, decode(t, body)["can_approve"])
	resp, _ = do(t, http.MethodPost, srv.URL+"/api/v1/oauth/requests/"+requestId+"/approve", userApiToken, "", "")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	resp, body = do(t, http.MethodGet, srv.URL+"/api/v1/oauth/requests/"+requestId, adminApiToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	consent := decode(t, body)
	assert.Equal(t, "Claude Code", consent["client_name"])
	assert.Equal(t, "localhost:51515", consent["redirect_host"])

	resp, body = do(t, http.MethodPost, srv.URL+"/api/v1/oauth/requests/"+requestId+"/approve", adminApiToken, "", "")
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	back, err := url.Parse(decode(t, body)["redirect_url"].(string))
	require.NoError(t, err)
	assert.Equal(t, "localhost:51515", back.Host)
	assert.Equal(t, "st", back.Query().Get("state"))

	resp, _ = do(t, http.MethodGet, srv.URL+"/api/v1/oauth/requests/"+requestId, adminApiToken, "", "")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a decided request is gone")

	// token exchange
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {back.Query().Get("code")},
		"redirect_uri":  {"http://localhost:51515/callback"},
		"code_verifier": {verifier},
		"client_id":     {clientId},
	}
	resp, body = do(t, http.MethodPost, srv.URL+"/api/v1/oauth/token", "", "application/x-www-form-urlencoded", form.Encode())
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	tokens := decode(t, body)
	accessToken := tokens["access_token"].(string)

	// replaying the code is an OAuth error, in the RFC 6749 shape
	resp, body = do(t, http.MethodPost, srv.URL+"/api/v1/oauth/token", "", "application/x-www-form-urlencoded", form.Encode())
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "invalid_grant", decode(t, body)["error"])

	// the MCP token opens the MCP endpoint...
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   srv.URL + "/api/v1/mcp",
		HTTPClient: &http.Client{Transport: bearer{accessToken}},
	}, nil)
	require.NoError(t, err)
	defer session.Close()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "system_overview"})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Content[0].(*mcp.TextContent).Text)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"active_sessions"`)

	// ...but not the REST API, and the API token does not open MCP
	resp, _ = do(t, http.MethodGet, srv.URL+"/api/v1/users/list", accessToken, "", "")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp, _ = do(t, http.MethodPost, srv.URL+"/api/v1/mcp", adminApiToken, "application/json", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// revoking ends the session
	resp, _ = do(t, http.MethodPost, srv.URL+"/api/v1/oauth/revoke", "", "application/x-www-form-urlencoded",
		url.Values{"token": {tokens["refresh_token"].(string)}, "client_id": {clientId}}.Encode())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp, _ = do(t, http.MethodPost, srv.URL+"/api/v1/mcp", accessToken, "application/json", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthorizeRejectsUnknownClientWithoutRedirect(t *testing.T) {
	srv := newMcpTestServer(t, true)
	resp, body := do(t, http.MethodGet, srv.URL+"/api/v1/oauth/authorize?client_id=mcp_nope&redirect_uri=https://evil.test/cb&response_type=code", "", "", "")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Location"))
	assert.Contains(t, string(body), "invalid_client")
}

func TestMcpDisabled(t *testing.T) {
	srv := newMcpTestServer(t, false)
	for _, path := range []string{"/api/v1/mcp", "/api/v1/oauth/protected-resource", "/.well-known/oauth-authorization-server"} {
		resp, _ := do(t, http.MethodGet, srv.URL+path, "", "", "")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
	}
	// the REST API is unaffected
	resp, _ := do(t, http.MethodGet, srv.URL+"/api/v1/users/list", adminApiToken, "", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
