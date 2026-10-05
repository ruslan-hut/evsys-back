package oauth

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
)

// Schemes a redirect URI may never use: they run code or read local data in
// the browser instead of handing the code to a client.
var forbiddenSchemes = []string{"javascript", "data", "file", "vbscript", "about", "blob"}

// validateRedirectUri accepts https URIs, http URIs on a loopback host (native
// clients such as Claude Code listen on localhost, RFC 8252 section 7.3) and
// private-use schemes of desktop apps (RFC 8252 section 7.1).
func validateRedirectUri(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a valid URI")
	}
	if !u.IsAbs() {
		return fmt.Errorf("must be an absolute URI")
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return fmt.Errorf("must not contain a fragment")
	}
	scheme := strings.ToLower(u.Scheme)
	switch {
	case scheme == "https":
		if u.Host == "" {
			return fmt.Errorf("https URI without host")
		}
	case scheme == "http":
		if !isLoopback(u.Hostname()) {
			return fmt.Errorf("http is only allowed for loopback hosts")
		}
	case slices.Contains(forbiddenSchemes, scheme):
		return fmt.Errorf("scheme %s is not allowed", scheme)
	}
	return nil
}

// matchRedirectUri compares a requested redirect URI with the registered ones:
// exactly, except that loopback http URIs match on any port, since a native
// client picks a free port at the time it signs in (RFC 8252 section 7.3).
func matchRedirectUri(registered []string, requested string) bool {
	if slices.Contains(registered, requested) {
		return true
	}
	req, err := url.Parse(requested)
	if err != nil || req.Scheme != "http" || !isLoopback(req.Hostname()) {
		return false
	}
	for _, r := range registered {
		reg, err := url.Parse(r)
		if err != nil || reg.Scheme != "http" {
			continue
		}
		if reg.Hostname() == req.Hostname() && reg.Path == req.Path && reg.RawQuery == req.RawQuery {
			return true
		}
	}
	return false
}

// knownRedirectHosts are the hosted MCP clients whose callbacks need no
// warning on the consent page.
var knownRedirectHosts = []string{"claude.ai", "claude.com"}

// isKnownRedirect reports whether a redirect URI goes to a loopback listener
// or a known hosted client.
func isKnownRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "http":
		return isLoopback(host)
	case "https":
		return isLoopback(host) || slices.Contains(knownRedirectHosts, host)
	}
	return false
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// redirectHost names where the code will go, for the consent page: the host
// for web redirects, the scheme for desktop apps.
func redirectHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Host != "" {
		return u.Host
	}
	return u.Scheme + ":"
}

// appendQuery adds the non-empty parameters to a URI, keeping the query it
// already has.
func appendQuery(raw string, params map[string]string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
