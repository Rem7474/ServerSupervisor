package synthetic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// SSLDB is the subset of database.DB methods needed by the SSL worker.
type SSLDB interface {
	ListEnabledSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error)
	UpdateSSLCertificateCheckResult(ctx context.Context, c models.SSLCertificate) error
}

const (
	sslCheckInterval = 6 * time.Hour
	sslDialTimeout   = 15 * time.Second
)

// RunSSLWorker runs the SSL/TLS expiration checker until ctx is cancelled.
// First check happens shortly after startup, then every sslCheckInterval.
func RunSSLWorker(ctx context.Context, db SSLDB) {
	// Initial run after a short delay so the database / network are ready.
	initial := time.NewTimer(30 * time.Second)
	defer initial.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-initial.C:
			checkAllCertificates(ctx, db)
			initial.Reset(sslCheckInterval)
		}
	}
}

func checkAllCertificates(ctx context.Context, db SSLDB) {
	certs, err := db.ListEnabledSSLCertificates(ctx)
	if err != nil {
		return
	}
	for _, c := range certs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result := checkCertificate(ctx, c)
		_ = db.UpdateSSLCertificateCheckResult(ctx, result)
	}
}

// CheckCertificate performs an on-demand TLS handshake and returns the updated
// certificate record. Exposed for the "force check" handler.
func CheckCertificate(ctx context.Context, c models.SSLCertificate) models.SSLCertificate {
	return checkCertificate(ctx, c)
}

func checkCertificate(ctx context.Context, c models.SSLCertificate) models.SSLCertificate {
	now := time.Now()
	c.LastCheckedAt = &now
	c.LastError = ""

	port := c.Port
	if port == 0 {
		port = 443
	}
	serverName := c.ServerName
	if serverName == "" {
		serverName = c.Host
	}

	dialCtx, cancel := context.WithTimeout(ctx, sslDialTimeout)
	defer cancel()

	dialer := &net.Dialer{Timeout: sslDialTimeout}
	addr := net.JoinHostPort(c.Host, strconv.Itoa(port))
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: serverName,
		// We always want to read the cert chain even if it's expired or invalid,
		// otherwise we can't report "expired N days ago".
		InsecureSkipVerify: true, //nolint:gosec
	})
	_ = dialCtx
	if err != nil {
		c.LastError = err.Error()
		return c
	}
	defer func() { _ = conn.Close() }()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		c.LastError = "no peer certificates returned"
		return c
	}
	leaf := state.PeerCertificates[0]

	notBefore := leaf.NotBefore
	notAfter := leaf.NotAfter
	c.ValidFrom = &notBefore
	c.ValidTo = &notAfter
	c.Issuer = leaf.Issuer.String()
	c.Subject = leaf.Subject.String()
	c.SerialNumber = leaf.SerialNumber.String()
	c.DNSNames = append([]string(nil), leaf.DNSNames...)
	if c.DNSNames == nil {
		c.DNSNames = []string{}
	}

	// Surface near-expiry as a non-fatal warning in last_error so the UI can show it.
	if remaining := time.Until(notAfter); remaining < 0 {
		c.LastError = fmt.Sprintf("certificate expired on %s", notAfter.Format(time.RFC3339))
	}
	return c
}

// Compile-time check: database.DB satisfies SSLDB.
var _ SSLDB = (*database.DB)(nil)
