package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"evsys-back/entity"
	databasemock "evsys-back/impl/database-mock"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRedirect = "https://claude.ai/api/mcp/auth_callback"
	testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-and-some-more"
)

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type fixture struct {
	svc   *Service
	db    *databasemock.MockDB
	admin *entity.User
	clock time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := databasemock.NewMockDB()
	admin := &entity.User{Username: "boss", Role: "admin", AccessLevel: 10, Token: "secret-api-token", Password: "hash"}
	db.SeedUser(admin)
	db.SeedUser(&entity.User{Username: "driver", Role: "user", AccessLevel: 1})
	svc, err := New(Config{
		PublicUrl:       "https://ev.example.com/",
		FrontendUrl:     "https://admin.example.com",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	f := &fixture{svc: svc, db: db, admin: admin, clock: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)}
	svc.now = func() time.Time { return f.clock }
	return f
}

func (f *fixture) register(t *testing.T, md *entity.OAuthClientMetadata) *entity.OAuthClientInformation {
	t.Helper()
	if md == nil {
		md = &entity.OAuthClientMetadata{ClientName: "Claude", RedirectUris: []string{testRedirect}}
	}
	info, err := f.svc.RegisterClient(context.Background(), md)
	require.NoError(t, err)
	return info
}

func (f *fixture) authorize(t *testing.T, clientId string) string {
	t.Helper()
	consentUrl, err := f.svc.Authorize(context.Background(), entity.OAuthAuthorizeRequest{
		ResponseType:        "code",
		ClientId:            clientId,
		RedirectUri:         testRedirect,
		State:               "xyz",
		CodeChallenge:       challenge(testVerifier),
		CodeChallengeMethod: "S256",
		Resource:            "https://ev.example.com/api/v1/mcp",
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(consentUrl, "https://admin.example.com/oauth/authorize?request="), consentUrl)
	u, _ := url.Parse(consentUrl)
	return u.Query().Get("request")
}

// approve runs the consent and returns the authorization code.
func (f *fixture) approve(t *testing.T, requestId string) string {
	t.Helper()
	redirect, err := f.svc.Approve(context.Background(), requestId, f.admin)
	require.NoError(t, err)
	u, err := url.Parse(redirect.RedirectUrl)
	require.NoError(t, err)
	assert.Equal(t, "claude.ai", u.Host)
	assert.Equal(t, "xyz", u.Query().Get("state"))
	assert.Equal(t, "https://ev.example.com/api/v1/oauth", u.Query().Get("iss"))
	return u.Query().Get("code")
}

func (f *fixture) tokens(t *testing.T, clientId string) *entity.OAuthTokenResponse {
	t.Helper()
	code := f.approve(t, f.authorize(t, clientId))
	resp, err := f.svc.Token(context.Background(), entity.OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectUri:  testRedirect,
		CodeVerifier: testVerifier,
		ClientId:     clientId,
	})
	require.NoError(t, err)
	return resp
}

func oauthCode(t *testing.T, err error) string {
	t.Helper()
	var oe *entity.OAuthError
	require.True(t, errors.As(err, &oe), "expected an OAuth error, got %v", err)
	return oe.Code
}

func TestMetadata(t *testing.T) {
	f := newFixture(t)
	assert.Equal(t, "https://ev.example.com/api/v1/oauth", f.svc.Issuer())
	assert.Equal(t, "https://ev.example.com/api/v1/mcp", f.svc.Resource())
	as := f.svc.AuthorizationServerMetadata()
	assert.Equal(t, f.svc.Issuer(), as["issuer"])
	assert.Equal(t, []string{"S256"}, as["code_challenge_methods_supported"])
	prm := f.svc.ProtectedResourceMetadata()
	assert.Equal(t, []string{f.svc.Issuer()}, prm["authorization_servers"])
}

func TestFullFlow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	assert.Equal(t, "none", client.TokenEndpointAuthMethod)
	assert.Empty(t, client.ClientSecret)

	requestId := f.authorize(t, client.ClientId)
	consent, err := f.svc.Consent(ctx, requestId, f.admin)
	require.NoError(t, err)
	assert.Equal(t, "Claude", consent.ClientName)
	assert.Equal(t, "claude.ai", consent.RedirectHost)
	assert.True(t, consent.CanApprove)

	code := f.approve(t, requestId)
	resp, err := f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType:    "authorization_code",
		Code:         code,
		RedirectUri:  testRedirect,
		CodeVerifier: testVerifier,
		ClientId:     client.ClientId,
	})
	require.NoError(t, err)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Equal(t, int64(3600), resp.ExpiresIn)
	assert.Equal(t, entity.OAuthScope, resp.Scope)

	user, token, err := f.svc.VerifyAccessToken(ctx, resp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, "boss", user.Username)
	assert.Empty(t, user.Token, "the API token must not travel with the MCP user")
	assert.Empty(t, user.Password)
	assert.Equal(t, client.ClientId, token.ClientId)

	// the refresh token is not an access token
	_, _, err = f.svc.VerifyAccessToken(ctx, resp.RefreshToken)
	assert.ErrorIs(t, err, entity.ErrInvalidToken)

	// the code is single use
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: client.ClientId,
	})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))
}

func TestConsentRequestIsSingleUse(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	requestId := f.authorize(t, client.ClientId)
	f.approve(t, requestId)
	_, err := f.svc.Approve(context.Background(), requestId, f.admin)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestConsentExpires(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	requestId := f.authorize(t, client.ClientId)
	f.clock = f.clock.Add(requestTTL + time.Second)
	_, err := f.svc.Consent(context.Background(), requestId, f.admin)
	assert.ErrorIs(t, err, entity.ErrNotFound)
	_, err = f.svc.Approve(context.Background(), requestId, f.admin)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestRegularUserCannotApprove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	requestId := f.authorize(t, client.ClientId)
	driver := &entity.User{Username: "driver", Role: "user"}

	consent, err := f.svc.Consent(ctx, requestId, driver)
	require.NoError(t, err)
	assert.False(t, consent.CanApprove)

	_, err = f.svc.Approve(ctx, requestId, driver)
	assert.ErrorIs(t, err, entity.ErrForbidden)
	// the refusal does not consume the request: an admin can still approve
	f.approve(t, requestId)
}

func TestDeny(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	requestId := f.authorize(t, client.ClientId)
	redirect, err := f.svc.Deny(context.Background(), requestId, f.admin)
	require.NoError(t, err)
	u, _ := url.Parse(redirect.RedirectUrl)
	assert.Equal(t, "access_denied", u.Query().Get("error"))
	assert.Equal(t, "xyz", u.Query().Get("state"))
	assert.Empty(t, u.Query().Get("code"))
}

func TestTokenRejectsWrongVerifierAndClient(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	other := f.register(t, nil)

	code := f.approve(t, f.authorize(t, client.ClientId))
	_, err := f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: strings.Repeat("a", 43), ClientId: client.ClientId,
	})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))

	code = f.approve(t, f.authorize(t, client.ClientId))
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: other.ClientId,
	})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))

	code = f.approve(t, f.authorize(t, client.ClientId))
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: client.ClientId,
		RedirectUri: "https://evil.example.com/cb",
	})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))

	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "authorization_code", ClientId: "mcp_unknown"})
	assert.Equal(t, "invalid_client", oauthCode(t, err))

	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "password", ClientId: client.ClientId})
	assert.Equal(t, "unsupported_grant_type", oauthCode(t, err))
}

func TestCodeExpires(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	code := f.approve(t, f.authorize(t, client.ClientId))
	f.clock = f.clock.Add(codeTTL + time.Second)
	_, err := f.svc.Token(context.Background(), entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: client.ClientId,
	})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))
}

func TestRefreshRotates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	first := f.tokens(t, client.ClientId)

	second, err := f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "refresh_token", RefreshToken: first.RefreshToken, ClientId: client.ClientId})
	require.NoError(t, err)
	assert.NotEqual(t, first.AccessToken, second.AccessToken)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken)
	_, _, err = f.svc.VerifyAccessToken(ctx, second.AccessToken)
	require.NoError(t, err)

	// a rotated refresh token is spent
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "refresh_token", RefreshToken: first.RefreshToken, ClientId: client.ClientId})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))

	// another client cannot use it
	other := f.register(t, nil)
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "refresh_token", RefreshToken: second.RefreshToken, ClientId: other.ClientId})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))
}

func TestAccessTokenExpires(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	resp := f.tokens(t, client.ClientId)
	f.clock = f.clock.Add(time.Hour + time.Second)
	_, _, err := f.svc.VerifyAccessToken(context.Background(), resp.AccessToken)
	assert.ErrorIs(t, err, entity.ErrInvalidToken)
}

func TestDemotedUserLosesAccess(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	resp := f.tokens(t, client.ClientId)

	f.admin.Role = "user"
	_, _, err := f.svc.VerifyAccessToken(ctx, resp.AccessToken)
	assert.ErrorIs(t, err, entity.ErrInvalidToken)
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{GrantType: "refresh_token", RefreshToken: resp.RefreshToken, ClientId: client.ClientId})
	assert.Equal(t, "invalid_grant", oauthCode(t, err))
}

func TestRevokeEndsGrant(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	resp := f.tokens(t, client.ClientId)

	require.NoError(t, f.svc.Revoke(ctx, resp.RefreshToken, client.ClientId, ""))
	_, _, err := f.svc.VerifyAccessToken(ctx, resp.AccessToken)
	assert.ErrorIs(t, err, entity.ErrInvalidToken)
	// unknown tokens are not an error
	assert.NoError(t, f.svc.Revoke(ctx, "evsys_at_nothing", client.ClientId, ""))
}

func TestConfidentialClient(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, &entity.OAuthClientMetadata{
		ClientName: "Desktop", RedirectUris: []string{testRedirect}, TokenEndpointAuthMethod: "client_secret_post",
	})
	require.NotEmpty(t, client.ClientSecret)
	require.NotNil(t, client.ClientSecretExpiresAt)

	code := f.approve(t, f.authorize(t, client.ClientId))
	_, err := f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: client.ClientId, ClientSecret: "wrong",
	})
	assert.Equal(t, "invalid_client", oauthCode(t, err))

	code = f.approve(t, f.authorize(t, client.ClientId))
	_, err = f.svc.Token(ctx, entity.OAuthTokenRequest{
		GrantType: "authorization_code", Code: code, CodeVerifier: testVerifier, ClientId: client.ClientId, ClientSecret: client.ClientSecret,
	})
	assert.NoError(t, err)
}

func TestAuthorizeErrors(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	client := f.register(t, nil)
	valid := entity.OAuthAuthorizeRequest{
		ResponseType: "code", ClientId: client.ClientId, RedirectUri: testRedirect, State: "s1",
		CodeChallenge: challenge(testVerifier), CodeChallengeMethod: "S256",
	}

	// errors that must not redirect
	bad := valid
	bad.ClientId = "mcp_unknown"
	_, err := f.svc.Authorize(ctx, bad)
	assert.Equal(t, "invalid_client", oauthCode(t, err))

	bad = valid
	bad.RedirectUri = "https://evil.example.com/cb"
	_, err = f.svc.Authorize(ctx, bad)
	assert.Equal(t, "invalid_request", oauthCode(t, err))

	// errors reported to the client's redirect URI
	redirectError := func(req entity.OAuthAuthorizeRequest) string {
		target, err := f.svc.Authorize(ctx, req)
		require.NoError(t, err)
		u, _ := url.Parse(target)
		require.Equal(t, "claude.ai", u.Host, target)
		assert.Equal(t, "s1", u.Query().Get("state"))
		return u.Query().Get("error")
	}
	bad = valid
	bad.CodeChallengeMethod = "plain"
	assert.Equal(t, "invalid_request", redirectError(bad))
	bad = valid
	bad.CodeChallenge = ""
	assert.Equal(t, "invalid_request", redirectError(bad))
	bad = valid
	bad.ResponseType = "token"
	assert.Equal(t, "unsupported_response_type", redirectError(bad))
	bad = valid
	bad.Resource = "https://other.example.com/mcp"
	assert.Equal(t, "invalid_target", redirectError(bad))

	// a single registered URI may be omitted
	ok := valid
	ok.RedirectUri = ""
	target, err := f.svc.Authorize(ctx, ok)
	require.NoError(t, err)
	assert.Contains(t, target, "https://admin.example.com/oauth/authorize?request=")
}

func TestRegisterValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := map[string]*entity.OAuthClientMetadata{
		"no redirect":        {ClientName: "x"},
		"plain http":         {RedirectUris: []string{"http://example.com/cb"}},
		"javascript":         {RedirectUris: []string{"javascript:alert(1)"}},
		"fragment":           {RedirectUris: []string{"https://example.com/cb#frag"}},
		"relative":           {RedirectUris: []string{"/callback"}},
		"implicit grant":     {RedirectUris: []string{testRedirect}, GrantTypes: []string{"implicit"}},
		"token response":     {RedirectUris: []string{testRedirect}, ResponseTypes: []string{"token"}},
		"unknown authmethod": {RedirectUris: []string{testRedirect}, TokenEndpointAuthMethod: "private_key_jwt"},
	}
	for name, md := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.RegisterClient(ctx, md)
			assert.Error(t, err)
		})
	}

	for _, uri := range []string{"http://localhost:33418/callback", "http://127.0.0.1/cb", "cursor://anysphere.cursor-mcp/oauth/callback"} {
		_, err := f.svc.RegisterClient(ctx, &entity.OAuthClientMetadata{RedirectUris: []string{uri}})
		assert.NoError(t, err, uri)
	}
}

func TestLoopbackRedirectMatchesAnyPort(t *testing.T) {
	registered := []string{"http://localhost:33418/callback", testRedirect}
	assert.True(t, matchRedirectUri(registered, "http://localhost:33418/callback"))
	assert.True(t, matchRedirectUri(registered, "http://localhost:51000/callback"))
	assert.False(t, matchRedirectUri(registered, "http://localhost:51000/other"))
	assert.False(t, matchRedirectUri(registered, "http://127.0.0.1:51000/callback"), "host must match")
	assert.False(t, matchRedirectUri(registered, "https://claude.ai/api/mcp/auth_callback/x"))
	assert.True(t, matchRedirectUri(registered, testRedirect))
}

func TestPendingRequestsAreBounded(t *testing.T) {
	f := newFixture(t)
	client := f.register(t, nil)
	for i := 0; i < maxPending; i++ {
		f.authorize(t, client.ClientId)
	}
	target, err := f.svc.Authorize(context.Background(), entity.OAuthAuthorizeRequest{
		ResponseType: "code", ClientId: client.ClientId, RedirectUri: testRedirect,
		CodeChallenge: challenge(testVerifier), CodeChallengeMethod: "S256",
	})
	require.NoError(t, err)
	assert.Contains(t, target, "error=temporarily_unavailable")

	// expired requests make room again
	f.clock = f.clock.Add(requestTTL + time.Second)
	f.authorize(t, client.ClientId)
}

func TestKnownRedirect(t *testing.T) {
	assert.True(t, isKnownRedirect("https://claude.ai/api/mcp/auth_callback"))
	assert.True(t, isKnownRedirect("https://claude.com/api/mcp/auth_callback"))
	assert.True(t, isKnownRedirect("http://localhost:33418/callback"))
	assert.True(t, isKnownRedirect("http://127.0.0.1:9/cb"))
	assert.False(t, isKnownRedirect("https://claude.ai.evil.example/cb"))
	assert.False(t, isKnownRedirect("https://evil.example/claude.ai"))
	assert.False(t, isKnownRedirect("cursor://anysphere.cursor-mcp/oauth/callback"))
}
