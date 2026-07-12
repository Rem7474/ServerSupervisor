package host

import "github.com/serversupervisor/server/internal/models"

// minDiskForecastSpanDays is the minimum history span required before a
// trend is trusted — a couple of noisy samples on a fresh install shouldn't
// produce an alarming "3 days until full" forecast.
const minDiskForecastSpanDays = 7.0

// forecastDiskDaysUntilFull fits a least-squares line to used_percent over
// time and extrapolates the number of days until the mount point would hit
// 100% at the current rate. Returns ok=false when there's not enough history,
// or the trend is flat/shrinking (nothing useful to forecast).
func forecastDiskDaysUntilFull(points []models.DiskMetrics) (days float64, ok bool) {
	if len(points) < 2 {
		return 0, false
	}

	first := points[0].Timestamp
	last := points[len(points)-1].Timestamp
	spanDays := last.Sub(first).Hours() / 24
	if spanDays < minDiskForecastSpanDays {
		return 0, false
	}

	var n, sumX, sumY, sumXY, sumXX float64
	for _, p := range points {
		x := p.Timestamp.Sub(first).Hours() / 24
		y := p.UsedPercent
		n++
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}

	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return 0, false
	}
	slopePerDay := (n*sumXY - sumX*sumY) / denom
	if slopePerDay <= 0 {
		return 0, false
	}

	current := points[len(points)-1].UsedPercent
	if current >= 100 {
		return 0, true
	}
	return (100 - current) / slopePerDay, true
}
