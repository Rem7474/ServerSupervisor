package synthetic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

func TestCertificateIssues(t *testing.T) {
	future := time.Now().Add(30 * 24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name      string
		notAfter  time.Time
		verifyErr error
		wantEmpty bool
		want      []string // exact expected messages, checked when non-nil
	}{
		{
			name:      "valid and trusted: no issues",
			notAfter:  future,
			wantEmpty: true,
		},
		{
			name:     "expired, no verify error",
			notAfter: past,
			want:     []string{"certificate expired on " + past.Format(time.RFC3339)},
		},
		{
			name:      "not expired but verify says expired: still reported once via notAfter, not duplicated",
			notAfter:  past,
			verifyErr: x509.CertificateInvalidError{Reason: x509.Expired, Detail: "irrelevant"},
			want:      []string{"certificate expired on " + past.Format(time.RFC3339)},
		},
		{
			name:      "verify expired-only error and cert not actually expired: suppressed entirely",
			notAfter:  future,
			verifyErr: x509.CertificateInvalidError{Reason: x509.Expired, Detail: "irrelevant"},
			wantEmpty: true,
		},
		{
			name:      "verify says untrusted: kept",
			notAfter:  future,
			verifyErr: x509.UnknownAuthorityError{},
		},
		{
			name:      "verify says invalid for a non-expiry reason: kept",
			notAfter:  future,
			verifyErr: x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign, Detail: "irrelevant"},
		},
		{
			name:      "verify error wrapped: still recognised as expired via errors.As",
			notAfter:  past,
			verifyErr: errors.Join(x509.CertificateInvalidError{Reason: x509.Expired}),
			want:      []string{"certificate expired on " + past.Format(time.RFC3339)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := certificateIssues(tt.notAfter, tt.verifyErr)
			if tt.wantEmpty {
				if len(got) != 0 {
					t.Fatalf("certificateIssues() = %v, want none", got)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("certificateIssues() = %v, want at least one issue", got)
			}
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if g == w {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("certificateIssues() = %v, want it to contain %q", got, w)
				}
			}
		})
	}
}

// selfSignedCert generates a minimal self-signed certificate valid for
// 127.0.0.1, expiring at notAfter.
func selfSignedCert(t *testing.T, notAfter time.Time) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// listenTLS starts a TLS listener serving cert on 127.0.0.1 and accepts
// (and discards) connections in the background until the test ends.
func listenTLS(t *testing.T, cert tls.Certificate) int {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				buf := make([]byte, 1)
				_, _ = conn.Read(buf)
			}()
		}
	}()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestCheckCertificate_SelfSignedIsReportedAsUntrusted(t *testing.T) {
	// X.509 time encoding has no sub-second precision, so the value read
	// back from the wire is truncated relative to what we generate it with.
	notAfter := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	port := listenTLS(t, selfSignedCert(t, notAfter))

	result := checkCertificate(context.Background(), models.SSLCertificate{
		ID:   "cert-1",
		Host: "127.0.0.1",
		Port: port,
	})

	if result.LastError == "" {
		t.Fatal("expected last_error to report the self-signed / untrusted chain, got none")
	}
	if strings.Contains(result.LastError, "expired") {
		t.Fatalf("cert is not expired, last_error should not mention it: %q", result.LastError)
	}
	if result.SerialNumber == "" || result.Issuer == "" || result.Subject == "" {
		t.Fatalf("expected certificate fields to be populated, got %+v", result)
	}
	if result.ValidTo == nil || !result.ValidTo.Equal(notAfter) {
		t.Fatalf("expected ValidTo %v, got %v", notAfter, result.ValidTo)
	}
}

func TestCheckCertificate_ExpiredSelfSignedReportsExpiry(t *testing.T) {
	notAfter := time.Now().Add(-24 * time.Hour)
	port := listenTLS(t, selfSignedCert(t, notAfter))

	result := checkCertificate(context.Background(), models.SSLCertificate{
		ID:   "cert-2",
		Host: "127.0.0.1",
		Port: port,
	})

	if !strings.Contains(result.LastError, "certificate expired on") {
		t.Fatalf("expected last_error to mention expiry, got %q", result.LastError)
	}
}

func TestCheckCertificate_DialFailureIsReported(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	_ = ln.Close() // free the port so the dial below fails with connection refused

	result := checkCertificate(context.Background(), models.SSLCertificate{
		ID:   "cert-3",
		Host: "127.0.0.1",
		Port: port,
	})

	if result.LastError == "" {
		t.Fatal("expected a dial error to be reported in last_error")
	}
}
