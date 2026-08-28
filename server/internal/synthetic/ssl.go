package synthetic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// SSLDB is the subset of database.DB methods needed by the SSL worker.
type SSLDB interface {
	ListEnabledSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error)
	UpdateSSLCertificateCheckResult(ctx context.Context, c models.SSLCertificate) error
	InsertSSLCertificateEventIfNew(ctx context.Context, ev models.SSLCertificateEvent) error
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
		slog.WarnContext(ctx, "ssl: failed to list enabled certificates", slog.Any("err", err))
		return
	}
	for _, c := range certs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result := checkCertificate(ctx, c)
		if err := db.UpdateSSLCertificateCheckResult(ctx, result); err != nil {
			slog.WarnContext(ctx, "ssl: failed to update certificate check result",
				slog.String("cert_id", result.ID), slog.Any("err", err))
		}
		if result.SerialNumber != "" {
			ev := models.SSLCertificateEvent{
				CertificateID: result.ID,
				SerialNumber:  result.SerialNumber,
				ValidFrom:     result.ValidFrom,
				ValidTo:       result.ValidTo,
				Issuer:        result.Issuer,
				Subject:       result.Subject,
			}
			if err := db.InsertSSLCertificateEventIfNew(ctx, ev); err != nil {
				slog.WarnContext(ctx, "ssl: failed to insert certificate event",
					slog.String("cert_id", result.ID), slog.Any("err", err))
			}
		}
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

	// The handshake above skipped verification entirely (InsecureSkipVerify)
	// so we could still read the chain of an expired/invalid endpoint —
	// otherwise that connection would just fail before we got here. That
	// means a wrong hostname or an untrusted CA would go completely
	// unreported. Verify independently now and fold anything beyond plain
	// expiry into last_error too.
	intermediates := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	_, verifyErr := leaf.Verify(x509.VerifyOptions{DNSName: serverName, Intermediates: intermediates})
	c.LastError = strings.Join(certificateIssues(notAfter, verifyErr), "; ")
	return c
}

// certificateIssues turns a certificate's expiry and an independent
// chain/hostname verification error (nil if it passed) into the list of
// problems to report in last_error. verifyErr's own "certificate has
// expired" case is suppressed since the notAfter check already reports that
// more clearly ("expired on <date>" vs x509's raw error text) — anything
// else from verifyErr (wrong hostname, untrusted CA, ...) is kept.
func certificateIssues(notAfter time.Time, verifyErr error) []string {
	var issues []string
	if remaining := time.Until(notAfter); remaining < 0 {
		issues = append(issues, fmt.Sprintf("certificate expired on %s", notAfter.Format(time.RFC3339)))
	}
	if verifyErr != nil {
		var invalidErr x509.CertificateInvalidError
		if !errors.As(verifyErr, &invalidErr) || invalidErr.Reason != x509.Expired {
			issues = append(issues, verifyErr.Error())
		}
	}
	return issues
}

// Compile-time check: database.DB satisfies SSLDB.
var _ SSLDB = (*database.DB)(nil)
