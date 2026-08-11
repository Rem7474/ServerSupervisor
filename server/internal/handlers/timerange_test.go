package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testContextWithQuery(query string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	return c
}

func TestParseTimeRange_DefaultsToPeriod(t *testing.T) {
	c := testContextWithQuery("")
	since, until, ok := parseTimeRange(c, "24h")
	if !ok {
		t.Fatal("expected ok=true for default period")
	}
	if !until.IsZero() {
		t.Errorf("expected zero until (no upper bound) in period mode, got %v", until)
	}
	wantSince := time.Now().Add(-24 * time.Hour)
	if since.Sub(wantSince).Abs() > time.Second {
		t.Errorf("expected since ~24h ago, got %v (want ~%v)", since, wantSince)
	}
}

func TestParseTimeRange_ExplicitPeriod(t *testing.T) {
	c := testContextWithQuery("period=168h")
	since, until, ok := parseTimeRange(c, "24h")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !until.IsZero() {
		t.Errorf("expected zero until, got %v", until)
	}
	wantSince := time.Now().Add(-168 * time.Hour)
	if since.Sub(wantSince).Abs() > time.Second {
		t.Errorf("expected since ~168h ago, got %v", since)
	}
}

func TestParseTimeRange_InvalidPeriod(t *testing.T) {
	c := testContextWithQuery("period=not-a-duration")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false for an invalid period")
	}
}

func TestParseTimeRange_CustomRangeValid(t *testing.T) {
	from := "2026-08-08T13:25:00Z"
	to := "2026-08-08T15:02:00Z"
	c := testContextWithQuery("from=" + from + "&to=" + to)
	since, until, ok := parseTimeRange(c, "24h")
	if !ok {
		t.Fatal("expected ok=true for a valid custom range")
	}
	wantSince, _ := time.Parse(time.RFC3339, from)
	wantUntil, _ := time.Parse(time.RFC3339, to)
	if !since.Equal(wantSince) {
		t.Errorf("expected since=%v, got %v", wantSince, since)
	}
	if !until.Equal(wantUntil) {
		t.Errorf("expected until=%v, got %v", wantUntil, until)
	}
}

func TestParseTimeRange_FromAfterToRejected(t *testing.T) {
	c := testContextWithQuery("from=2026-08-08T15:02:00Z&to=2026-08-08T13:25:00Z")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false when from is after to")
	}
}

func TestParseTimeRange_EqualFromToRejected(t *testing.T) {
	c := testContextWithQuery("from=2026-08-08T13:25:00Z&to=2026-08-08T13:25:00Z")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false when from equals to")
	}
}

func TestParseTimeRange_RangeTooWideRejected(t *testing.T) {
	c := testContextWithQuery("from=2026-01-01T00:00:00Z&to=2026-06-01T00:00:00Z")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false for a range wider than 90 days")
	}
}

func TestParseTimeRange_PartialRangeRejected(t *testing.T) {
	c := testContextWithQuery("from=2026-08-08T13:25:00Z")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false when only from is provided")
	}
	c2 := testContextWithQuery("to=2026-08-08T13:25:00Z")
	_, _, ok2 := parseTimeRange(c2, "24h")
	if ok2 {
		t.Fatal("expected ok=false when only to is provided")
	}
}

func TestParseTimeRange_InvalidRFC3339Rejected(t *testing.T) {
	c := testContextWithQuery("from=2026-08-08&to=2026-08-08T15:02:00Z")
	_, _, ok := parseTimeRange(c, "24h")
	if ok {
		t.Fatal("expected ok=false for a non-RFC3339 from (date-only, no time component)")
	}
}

func TestParseTimeRange_FutureToIsClamped(t *testing.T) {
	from := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	to := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	c := testContextWithQuery("from=" + from + "&to=" + to)
	_, until, ok := parseTimeRange(c, "24h")
	if !ok {
		t.Fatal("expected ok=true, a future `to` is clamped rather than rejected")
	}
	if until.After(time.Now()) {
		t.Errorf("expected until to be clamped to now, got %v", until)
	}
}
