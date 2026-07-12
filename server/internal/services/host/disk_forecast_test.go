package host

import (
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

func diskPoint(daysAgo int, usedPercent float64) models.DiskMetrics {
	return models.DiskMetrics{
		Timestamp:   time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour),
		UsedPercent: usedPercent,
	}
}

func TestForecastDiskDaysUntilFull(t *testing.T) {
	t.Run("rising trend over enough history forecasts a positive day count", func(t *testing.T) {
		// 10 days of history, +2%/day: 60% -> 78% over 9 days.
		points := []models.DiskMetrics{
			diskPoint(9, 60), diskPoint(6, 66), diskPoint(3, 72), diskPoint(0, 78),
		}
		days, ok := forecastDiskDaysUntilFull(points)
		if !ok {
			t.Fatal("expected a forecast, got ok=false")
		}
		// (100-78)/2 = 11 days, generous tolerance for the least-squares fit.
		if days < 9 || days > 13 {
			t.Errorf("days = %.2f, want ~11", days)
		}
	})

	t.Run("too few points yields no forecast", func(t *testing.T) {
		if _, ok := forecastDiskDaysUntilFull([]models.DiskMetrics{diskPoint(0, 80)}); ok {
			t.Error("expected ok=false with a single point")
		}
		if _, ok := forecastDiskDaysUntilFull(nil); ok {
			t.Error("expected ok=false with no points")
		}
	})

	t.Run("history shorter than the minimum span yields no forecast", func(t *testing.T) {
		points := []models.DiskMetrics{diskPoint(2, 60), diskPoint(0, 70)}
		if _, ok := forecastDiskDaysUntilFull(points); ok {
			t.Error("expected ok=false with only 2 days of history")
		}
	})

	t.Run("flat usage yields no forecast", func(t *testing.T) {
		points := []models.DiskMetrics{diskPoint(9, 60), diskPoint(6, 60), diskPoint(3, 60), diskPoint(0, 60)}
		if _, ok := forecastDiskDaysUntilFull(points); ok {
			t.Error("expected ok=false for a flat trend")
		}
	})

	t.Run("shrinking usage yields no forecast", func(t *testing.T) {
		points := []models.DiskMetrics{diskPoint(9, 80), diskPoint(6, 75), diskPoint(3, 70), diskPoint(0, 65)}
		if _, ok := forecastDiskDaysUntilFull(points); ok {
			t.Error("expected ok=false for a shrinking trend")
		}
	})

	t.Run("already full returns zero days", func(t *testing.T) {
		points := []models.DiskMetrics{diskPoint(9, 90), diskPoint(0, 100)}
		days, ok := forecastDiskDaysUntilFull(points)
		if !ok {
			t.Fatal("expected ok=true when already at 100%")
		}
		if days != 0 {
			t.Errorf("days = %.2f, want 0", days)
		}
	})
}
