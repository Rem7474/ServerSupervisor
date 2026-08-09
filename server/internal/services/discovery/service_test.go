package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	hosts []models.Host
}

func (f *fakeRepo) GetAllHosts(context.Context) ([]models.Host, error) { return f.hosts, nil }

func newSvc(repo Repository, ping Pinger) *Service {
	return &Service{repo: repo, ping: ping}
}

func alwaysDown(_ context.Context, _ string) (bool, int, error) {
	return false, 0, errors.New("timeout")
}

func TestScan_RejectsInvalidCIDR(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "not-a-cidr")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("expected apperr 400, got %v", err)
	}
}

func TestScan_RejectsIPv6(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "2001:db8::/120")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("expected apperr 400 for IPv6, got %v", err)
	}
}

func TestScan_RejectsTooLargeNetwork(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "10.0.0.0/16")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("expected apperr 400 for a /16, got %v", err)
	}
}

func TestScan_RejectsTooSmallNetwork(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "10.0.0.0/31")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("expected apperr 400 for a /31, got %v", err)
	}
}

func TestScan_SlashThirtyExcludesNetworkAndBroadcast(t *testing.T) {
	results, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "10.0.0.0/30")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// /30 = 10.0.0.0..10.0.0.3; usable = .1 and .2 only.
	if len(results) != 2 {
		t.Fatalf("expected 2 usable addresses, got %d: %+v", len(results), results)
	}
	if results[0].IPAddress != "10.0.0.1" || results[1].IPAddress != "10.0.0.2" {
		t.Fatalf("unexpected addresses: %+v", results)
	}
}

func TestScan_MarksRespondingAndRegisteredHosts(t *testing.T) {
	repo := &fakeRepo{hosts: []models.Host{{ID: "h1", Name: "web", IPAddress: "10.0.0.2"}}}
	ping := func(_ context.Context, target string) (bool, int, error) {
		if target == "10.0.0.1" {
			return true, 5, nil
		}
		return false, 0, errors.New("no reply")
	}
	results, err := newSvc(repo, ping).Scan(context.Background(), "10.0.0.0/30")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !results[0].Responded || results[0].LatencyMs != 5 {
		t.Errorf(".1 should have responded with 5ms latency, got %+v", results[0])
	}
	if results[0].AlreadyRegistered {
		t.Errorf(".1 is not a known host, got %+v", results[0])
	}
	if results[1].Responded {
		t.Errorf(".2 should not have responded, got %+v", results[1])
	}
	if !results[1].AlreadyRegistered || results[1].ExistingHostID != "h1" || results[1].ExistingHostName != "web" {
		t.Errorf(".2 should be marked as the existing host h1/web, got %+v", results[1])
	}
}

func TestScan_ResultsSortedByIP(t *testing.T) {
	results, err := newSvc(&fakeRepo{}, alwaysDown).Scan(context.Background(), "10.0.0.0/29")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if !ipLess(results[i-1].IPAddress, results[i].IPAddress) {
			t.Fatalf("results not sorted at index %d: %+v", i, results)
		}
	}
}
