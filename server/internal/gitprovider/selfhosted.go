package gitprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// Default public instances used when a gitlab/gitea tracker has no base URL.
const (
	DefaultGitLabBaseURL = "https://gitlab.com"
	DefaultGiteaBaseURL  = "https://codeberg.org"
)

// Outbound policy for user-supplied base URLs.
//
// Self-hosted GitLab/Gitea instances usually live on the operator's LAN, so
// private ranges (RFC 1918, ULA) are deliberately allowed. What is refused:
// non-http(s) schemes, credentials embedded in the URL, loopback (the server
// itself), link-local (cloud metadata endpoints), unspecified and multicast
// addresses. The address check runs on the literal host when the URL is
// validated and again on every resolved address when the connection is
// opened, so a hostname resolving to a refused address (DNS rebinding,
// redirects) is rejected as well.

var errBlockedAddress = errors.New("address not allowed for a git provider base URL")

func blockedAddr(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified()
}

// NormalizeBaseURL validates a self-hosted instance base URL and returns it
// without trailing slash. An empty input returns "" (caller applies the
// provider default). A sub-path (https://host/gitlab) is kept.
func NormalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return "", errors.New("host is required")
	}
	if u.User != nil {
		return "", errors.New("credentials in the URL are not allowed")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("query string and fragment are not allowed")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return "", errBlockedAddress
	}
	if ip, perr := netip.ParseAddr(host); perr == nil && blockedAddr(ip) {
		return "", errBlockedAddress
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u.String(), nil
}

// guardedDialControl rejects connections to refused addresses at dial time.
func guardedDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if blockedAddr(ip) {
		return errBlockedAddress
	}
	return nil
}

// newGuardedHTTPClient builds the HTTP client for self-hosted instances: no
// proxy (a proxy would turn the dial check into a check of the proxy address),
// no redirect following (custom auth headers survive redirects, and the target
// would escape the validated base URL), dial-time address guard.
func newGuardedHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: guardedDialControl}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// restAPI is the small JSON-over-HTTP helper shared by the GitLab and Gitea
// clients.
type restAPI struct {
	provider   string
	baseURL    string
	token      string
	authHeader func(token string) (name, value string)
	http       *http.Client
}

// getJSON GETs baseURL+path and decodes the JSON body into out. A non-200
// status becomes an *APIError.
func (r *restAPI) getJSON(path string, out any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, r.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if r.token != "" {
		name, value := r.authHeader(r.token)
		req.Header.Set(name, value)
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return &APIError{Status: resp.StatusCode, Provider: r.provider}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func isNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}
