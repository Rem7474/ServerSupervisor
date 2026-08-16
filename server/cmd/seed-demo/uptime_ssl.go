package main

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type demoProbe struct {
	Name       string
	Type       string
	Target     string
	OKRun      int  // number of successful checks in the history, oldest first
	FailRun    int  // number of failing checks appended after OKRun
	FinalState bool // true = probe ends "up", false = ends "down" (edge case)
}

var demoProbes = []demoProbe{
	{Name: "Demo - Site web (HTTP)", Type: "http", Target: "https://demo.example.com", OKRun: 20, FailRun: 0, FinalState: true},
	{Name: "Demo - API interne (TCP)", Type: "tcp", Target: "10.0.10.21:8000", OKRun: 20, FailRun: 0, FinalState: true},
	// The "outage" edge case: 5 consecutive failures, currently down.
	{Name: "Demo - Ancien service (down)", Type: "http", Target: "https://legacy.demo.example.com", OKRun: 15, FailRun: 5, FinalState: false},
}

type demoCert struct {
	Name      string
	Host      string
	ExpiresIn time.Duration // negative = already expired
}

var demoCerts = []demoCert{
	{Name: "Demo - Certificat site web", Host: "demo.example.com", ExpiresIn: 90 * 24 * time.Hour},
	// The explicit "certificate expiring soon" edge case (crosses the <=7d warning threshold).
	{Name: "Demo - Certificat API", Host: "api.demo.example.com", ExpiresIn: 5 * 24 * time.Hour},
	{Name: "Demo - Certificat legacy", Host: "legacy.demo.example.com", ExpiresIn: -3 * 24 * time.Hour},
}

func seedUptimeSSL(ctx context.Context, db *database.DB) error {
	if err := seedProbes(ctx, db); err != nil {
		return err
	}
	return seedCerts(ctx, db)
}

func seedProbes(ctx context.Context, db *database.DB) error {
	existing, err := db.ListUptimeProbes(ctx)
	if err != nil {
		return err
	}
	byName := map[string]string{}
	for _, p := range existing {
		byName[p.Name] = p.ID
	}

	for _, d := range demoProbes {
		req := models.UptimeProbe{
			Name: d.Name, Type: d.Type, Target: d.Target, IntervalSec: 60, TimeoutSec: 10,
			ExpectedStatus: 200, FollowRedirects: true, VerifyTLS: true, Enabled: true,
		}
		var probeID string
		if id, ok := byName[d.Name]; ok {
			probeID = id
			req.ID = id
			if err := db.UpdateUptimeProbe(ctx, req); err != nil {
				return err
			}
		} else {
			created, err := db.CreateUptimeProbe(ctx, req)
			if err != nil {
				return err
			}
			probeID = created.ID
		}

		// Reset the cached counter/history before replaying — RecordUptimeProbeResult
		// increments consecutive_failures off the row's *current* value, so without
		// this reset a re-seed would push a "down" probe's counter higher every run.
		if _, err := db.Exec(ctx, `UPDATE uptime_probes SET consecutive_failures = 0 WHERE id = $1`, probeID); err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `DELETE FROM uptime_probe_results WHERE probe_id = $1`, probeID); err != nil {
			return err
		}

		total := d.OKRun + d.FailRun
		start := anchor.Add(-time.Duration(total) * 5 * time.Minute)
		for i := 0; i < total; i++ {
			success := i < d.OKRun
			checkedAt := start.Add(time.Duration(i) * 5 * time.Minute)
			result := models.UptimeProbeResult{ProbeID: probeID, CheckedAt: checkedAt, Success: success, LatencyMs: 45}
			if success {
				code := 200
				result.StatusCode = &code
			} else {
				result.Error = "connection timed out"
			}
			if err := db.RecordUptimeProbeResult(ctx, result); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedCerts(ctx context.Context, db *database.DB) error {
	existing, err := db.ListSSLCertificates(ctx)
	if err != nil {
		return err
	}
	byName := map[string]string{}
	for _, c := range existing {
		byName[c.Name] = c.ID
	}

	for _, d := range demoCerts {
		var certID string
		if id, ok := byName[d.Name]; ok {
			certID = id
		} else {
			created, err := db.CreateSSLCertificate(ctx, models.SSLCertificate{
				Name: d.Name, Host: d.Host, Port: 443, Enabled: true,
			})
			if err != nil {
				return err
			}
			certID = created.ID
		}

		validFrom := anchor.Add(-60 * 24 * time.Hour)
		validTo := anchor.Add(d.ExpiresIn)
		if err := db.UpdateSSLCertificateCheckResult(ctx, models.SSLCertificate{
			ID: certID, LastCheckedAt: &anchor, ValidFrom: &validFrom, ValidTo: &validTo,
			Issuer: "Let's Encrypt", Subject: "CN=" + d.Host, SerialNumber: "DEMO" + certID[:8],
			DNSNames: []string{d.Host},
		}); err != nil {
			return err
		}
	}
	return nil
}
