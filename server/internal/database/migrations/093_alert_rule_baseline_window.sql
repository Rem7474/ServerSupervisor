-- Adds the rolling-average window (seconds) used by the new
-- bandwidth_vs_rolling_avg alert metric (1h/6h/24h presets) — see
-- internal/alerts/metrics.go's GetMetricValue and
-- internal/services/alertrule/service.go's validateBaselineWindow.
-- Nullable: every other metric leaves this unset; the engine defaults to
-- 3600 (1h) when NULL so existing rules of other metrics are unaffected.
ALTER TABLE alert_rules ADD COLUMN baseline_window_seconds INT;

-- Same field on rule templates (bandwidth_vs_rolling_avg is a per-host agent
-- metric, so it's templatable — see isTemplatableMetric — and a saved
-- template needs to carry its window through ApplyTemplate).
ALTER TABLE alert_rule_templates ADD COLUMN baseline_window_seconds INT;
