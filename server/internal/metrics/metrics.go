// Package metrics exposes the server's own health in Prometheus text
// exposition format (self-observability: HTTP traffic, runtime, DB pool,
// connected agents, command and notification dispatch).
//
// Everything lives on a private registry rather than the client_golang
// default one, so nothing a dependency registers globally leaks into the
// scrape output. Label values are always drawn from bounded sets (gin route
// templates, status classes, module names, channel names) — never raw paths,
// host IDs or usernames — to keep series cardinality fixed.
package metrics

import (
	"database/sql"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const namespace = "serversupervisor"

var registry = prometheus.NewRegistry()

var (
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "HTTP requests handled, by method, route template and status class.",
	}, []string{"method", "route", "status_class"})

	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency, by method and route template.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})

	httpInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "http_requests_in_flight",
		Help:      "HTTP requests currently being served.",
	})

	commandsDispatched = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "commands_dispatched_total",
		Help:      "Remote commands queued for agents, by module and outcome (queued or error).",
	}, []string{"module", "outcome"})

	notificationsSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "notifications_total",
		Help:      "Notification deliveries attempted, by channel and outcome (success or error).",
	}, []string{"channel", "outcome"})
)

// Sources are the live values read at scrape time. Any nil func reports 0.
type Sources struct {
	AgentsConnected func() int
	WSClients       func() int
	DBStats         func() sql.DBStats
}

var (
	srcMu   sync.RWMutex
	sources Sources
)

// SetSources installs the scrape-time value providers. Safe to call more than
// once (the last call wins), which matters for tests that build several routers.
func SetSources(s Sources) {
	srcMu.Lock()
	sources = s
	srcMu.Unlock()
}

func currentSources() Sources {
	srcMu.RLock()
	defer srcMu.RUnlock()
	return sources
}

type liveCollector struct {
	agents, wsClients                                       *prometheus.Desc
	dbOpen, dbInUse, dbIdle, dbMax, dbWaitCount, dbWaitSecs *prometheus.Desc
}

func newLiveCollector() *liveCollector {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(namespace, "", name), help, nil, nil)
	}
	return &liveCollector{
		agents:      d("agents_connected", "Agents holding a live WebSocket push connection."),
		wsClients:   d("websocket_clients", "Browser clients connected to the notification stream."),
		dbOpen:      d("db_connections_open", "Open database connections (in use + idle)."),
		dbInUse:     d("db_connections_in_use", "Database connections currently in use."),
		dbIdle:      d("db_connections_idle", "Idle database connections."),
		dbMax:       d("db_connections_max_open", "Configured maximum of open database connections (0 = unlimited)."),
		dbWaitCount: d("db_connection_wait_total", "Total number of connections waited for."),
		dbWaitSecs:  d("db_connection_wait_seconds_total", "Total time blocked waiting for a new database connection."),
	}
}

func (c *liveCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.agents, c.wsClients, c.dbOpen, c.dbInUse, c.dbIdle, c.dbMax, c.dbWaitCount, c.dbWaitSecs} {
		ch <- d
	}
}

func (c *liveCollector) Collect(ch chan<- prometheus.Metric) {
	s := currentSources()
	gauge := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v)
	}
	counter := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}
	var agents, clients int
	if s.AgentsConnected != nil {
		agents = s.AgentsConnected()
	}
	if s.WSClients != nil {
		clients = s.WSClients()
	}
	gauge(c.agents, float64(agents))
	gauge(c.wsClients, float64(clients))
	var st sql.DBStats
	if s.DBStats != nil {
		st = s.DBStats()
	}
	gauge(c.dbOpen, float64(st.OpenConnections))
	gauge(c.dbInUse, float64(st.InUse))
	gauge(c.dbIdle, float64(st.Idle))
	gauge(c.dbMax, float64(st.MaxOpenConnections))
	counter(c.dbWaitCount, float64(st.WaitCount))
	counter(c.dbWaitSecs, st.WaitDuration.Seconds())
}

func init() {
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequests, httpDuration, httpInFlight,
		commandsDispatched, notificationsSent,
		newLiveCollector(),
	)
}

// CommandDispatched records one remote command creation attempt.
func CommandDispatched(module string, err error) {
	commandsDispatched.WithLabelValues(module, outcome(err, "queued")).Inc()
}

// NotificationSent records one delivery attempt on a channel (smtp, ntfy, notify).
func NotificationSent(channel string, err error) {
	notificationsSent.WithLabelValues(channel, outcome(err, "success")).Inc()
}

func outcome(err error, ok string) string {
	if err != nil {
		return "error"
	}
	return ok
}
