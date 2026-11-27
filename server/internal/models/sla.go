package models

import "time"

// ========== SLA / SLO reporting ==========

// SLA window status values.
const (
	SLAStatusMet      = "met"
	SLAStatusAtRisk   = "at_risk"
	SLAStatusBreached = "breached"
	SLAStatusNoData   = "no_data"
)

// UptimeDailyCount is one probe's check counts for one UTC day.
type UptimeDailyCount struct {
	ProbeID     string    `json:"probe_id"`
	Day         time.Time `json:"day"`
	TotalChecks int       `json:"total_checks"`
	UpChecks    int       `json:"up_checks"`
}

// SLAWindow is a probe's measured availability over a sliding window of
// calendar days (UTC), compared against its objective.
type SLAWindow struct {
	Days int `json:"days"`
	// CoveredDays is how many days of the window actually hold check results
	// (a new probe, a disabled period or a history shorter than the window
	// all lower it): the percentages below only describe those days.
	CoveredDays int `json:"covered_days"`
	TotalChecks int `json:"total_checks"`
	UpChecks    int `json:"up_checks"`
	// UptimePercent is nil when the window holds no check at all.
	UptimePercent *float64 `json:"uptime_percent"`
	// BudgetRemainingPercent is the share of the error budget (the failed
	// checks the objective tolerates) still unused, clamped to [0, 100]. Nil
	// without data.
	BudgetRemainingPercent *float64 `json:"budget_remaining_percent"`
	// AllowedDowntimeMinutes is the downtime the objective tolerates over the
	// full window length, for orientation ("99.9 % over 30 days = 43 min").
	AllowedDowntimeMinutes float64 `json:"allowed_downtime_minutes"`
	Status                 string  `json:"status"` // met | at_risk | breached | no_data
}

// ProbeSLA is the SLA report of one uptime probe.
type ProbeSLA struct {
	ProbeID   string  `json:"probe_id"`
	Name      string  `json:"name"`
	Enabled   bool    `json:"enabled"`
	SLOTarget float64 `json:"slo_target"`
	// DataSince is the first UTC day (within the longest window) holding
	// results; nil when there is none.
	DataSince *time.Time  `json:"data_since"`
	Windows   []SLAWindow `json:"windows"`
}

// SLAReport is the response of GET /uptime/sla.
type SLAReport struct {
	GeneratedAt time.Time  `json:"generated_at"`
	WindowDays  []int      `json:"window_days"`
	Probes      []ProbeSLA `json:"probes"`
}

// ========== Public status page ==========

// Public status values shown on the status page.
const (
	PublicStatusOperational   = "operational"
	PublicStatusPartialOutage = "partial_outage"
	PublicStatusMajorOutage   = "major_outage"
	PublicStatusUnknown       = "unknown"
)

// StatusPageService is one probe an admin published on the status page.
type StatusPageService struct {
	ProbeID    string `json:"probe_id"`
	PublicName string `json:"public_name"`
}

// StatusPageConfig is the admin-side configuration of the public status page.
type StatusPageConfig struct {
	Enabled     bool                `json:"enabled"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Services    []StatusPageService `json:"services"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// StatusPageConfigRequest is the body of PUT /status-page. Services are
// shown in the order given.
type StatusPageConfigRequest struct {
	Enabled     bool                `json:"enabled"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Services    []StatusPageService `json:"services"`
}

// PublishedProbe joins a published service with the live state of its probe.
// Server-side only: never serialized to the public endpoint as is.
type PublishedProbe struct {
	ProbeID    string `json:"-"`
	PublicName string `json:"-"`
	Enabled    bool   `json:"-"`
	LastStatus string `json:"-"`
}

// UptimeDownPeriod is a run of consecutive failed checks of one probe.
// ResolvedAt is the first successful check after it, nil while ongoing.
type UptimeDownPeriod struct {
	ProbeID    string     `json:"-"`
	StartedAt  time.Time  `json:"-"`
	ResolvedAt *time.Time `json:"-"`
}

// PublicStatusDay is one day of a service's 90-day bar.
type PublicStatusDay struct {
	Date string `json:"date"` // YYYY-MM-DD (UTC)
	// UptimePercent is nil for a day without any check.
	UptimePercent *float64 `json:"uptime_percent"`
}

// PublicStatusService is a published service as the public sees it: its
// public name and availability only.
type PublicStatusService struct {
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	UptimePercent90d *float64          `json:"uptime_percent_90d"`
	Days             []PublicStatusDay `json:"days"`
}

// PublicStatusIncident is a generic outage record: which public service, when
// it started and when it was resolved (nil while ongoing). No cause or error.
type PublicStatusIncident struct {
	Service    string     `json:"service"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

// PublicStatusPage is the response of the unauthenticated GET
// /api/public/status endpoint.
type PublicStatusPage struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	GeneratedAt time.Time              `json:"generated_at"`
	Services    []PublicStatusService  `json:"services"`
	Incidents   []PublicStatusIncident `json:"incidents"`
}
