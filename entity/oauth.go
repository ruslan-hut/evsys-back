package entity

import (
	"errors"
	"time"
)

// OAuthScope is the only scope there is: read access to everything an admin
// or operator can see.
const OAuthScope = "evsys:read"

// OAuth token kinds stored in the oauth_tokens collection.
const (
	OAuthAccessToken  = "access"
	OAuthRefreshToken = "refresh"
)

var (
	// ErrInvalidToken is returned for any bearer token that does not grant
	// access, whatever the reason.
	ErrInvalidToken = errors.New("invalid token")
	// ErrForbidden is returned when a signed-in user may not grant MCP access.
	ErrForbidden = errors.New("only admins and operators can connect MCP clients")
)

// OAuthError is an OAuth error response (RFC 6749 section 5.2).
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
	Status      int    `json:"-"`
}

func (e *OAuthError) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

// OAuthClientMetadata is a dynamic client registration request (RFC 7591).
type OAuthClientMetadata struct {
	RedirectUris            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

// OAuthClientInformation is the registration response.
type OAuthClientInformation struct {
	ClientId                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIdIssuedAt        int64    `json:"client_id_issued_at"`
	ClientSecretExpiresAt   *int64   `json:"client_secret_expires_at,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectUris            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
}

// OAuthAuthorizeRequest holds the query parameters of the authorization
// endpoint.
type OAuthAuthorizeRequest struct {
	ResponseType        string
	ClientId            string
	RedirectUri         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

// OAuthTokenRequest holds the form parameters of the token endpoint, with the
// client credentials taken from either the form or HTTP Basic authentication.
type OAuthTokenRequest struct {
	GrantType    string
	Code         string
	RedirectUri  string
	CodeVerifier string
	RefreshToken string
	ClientId     string
	ClientSecret string
	Resource     string
}

type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// OAuthClient is an OAuth client registered through dynamic client
// registration (RFC 7591), typically an MCP client such as Claude.
// SecretHash is empty for public clients, which prove possession with PKCE
// alone.
type OAuthClient struct {
	ClientId                string    `json:"client_id" bson:"client_id"`
	ClientName              string    `json:"client_name" bson:"client_name"`
	RedirectUris            []string  `json:"redirect_uris" bson:"redirect_uris"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method" bson:"token_endpoint_auth_method"`
	SecretHash              string    `json:"-" bson:"secret_hash,omitempty"`
	CreatedAt               time.Time `json:"created_at" bson:"created_at"`
}

// OAuthToken is an issued access or refresh token. Only the SHA-256 hash of
// the token is stored, so a database dump does not hand out live tokens.
// Tokens issued from one consent share a GrantId, which is what revocation
// deletes by.
type OAuthToken struct {
	Hash      string    `bson:"hash"`
	Kind      string    `bson:"kind"`
	GrantId   string    `bson:"grant_id"`
	ClientId  string    `bson:"client_id"`
	Username  string    `bson:"username"`
	Scope     string    `bson:"scope"`
	Resource  string    `bson:"resource"`
	ExpiresAt time.Time `bson:"expires_at"`
	CreatedAt time.Time `bson:"created_at"`
}

// OAuthConsent describes a pending authorization request to the consent page
// in evsys-front.
type OAuthConsent struct {
	RequestId    string    `json:"request_id"`
	ClientId     string    `json:"client_id"`
	ClientName   string    `json:"client_name"`
	RedirectUri  string    `json:"redirect_uri"`
	RedirectHost string    `json:"redirect_host"`
	Scope        string    `json:"scope"`
	Resource     string    `json:"resource"`
	ExpiresAt    time.Time `json:"expires_at"`
	Username     string    `json:"username"`
	// KnownClient is true when the code goes back to a local listener (Claude
	// Code and other CLI clients) or to a known hosted client (Claude). For
	// any other destination the page warns: client names are self-declared,
	// so anyone can register "Claude" and send an admin the consent link.
	KnownClient bool `json:"known_client"`
	// CanApprove is false when the signed-in user's role does not allow MCP
	// access; the page then explains why instead of offering the button.
	CanApprove bool `json:"can_approve"`
}

// OAuthRedirect is the answer to a consent decision: the URL the browser must
// navigate to, carrying either the authorization code or the error.
type OAuthRedirect struct {
	RedirectUrl string `json:"redirect_url"`
}
