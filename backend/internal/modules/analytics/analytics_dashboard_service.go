package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
)

// DashboardServiceInterface defines the dashboard read endpoints (PRD §3.4.2
// Phase B). Separate from ServiceInterface so the ingest service stays focused;
// both share the same *Repository. All methods take an inclusive-from /
// exclusive-to UTC day window.
type DashboardServiceInterface interface {
	TrafficReport(ctx context.Context, from, to time.Time) (*models.TrafficReport, error)
	SalesReport(ctx context.Context, from, to time.Time) (*models.SalesReport, error)
	FunnelReport(ctx context.Context, from, to time.Time) (*models.FunnelReport, error)
}

// DashboardService reads dashboard data. It does NOT compute conversions itself
// beyond the funnel (the rest are direct repo results); the funnel conversion
// rates are derived here from the repo totals (a DB-side FILTER is cleaner for
// the raw counts, but the percentage is presentation logic → belongs here).
type DashboardService struct {
	repo DashboardRepo
}

// NewDashboardService constructs the dashboard read service.
func NewDashboardService(repo DashboardRepo) DashboardServiceInterface {
	return &DashboardService{repo: repo}
}

func (s *DashboardService) TrafficReport(ctx context.Context, from, to time.Time) (*models.TrafficReport, error) {
	rep, err := s.repo.Traffic(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.TrafficReport: %w", err)
	}
	return rep, nil
}

func (s *DashboardService) SalesReport(ctx context.Context, from, to time.Time) (*models.SalesReport, error) {
	rep, err := s.repo.Sales(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.SalesReport: %w", err)
	}
	return rep, nil
}

func (s *DashboardService) FunnelReport(ctx context.Context, from, to time.Time) (*models.FunnelReport, error) {
	rep, err := s.repo.Funnel(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.FunnelReport: %w", err)
	}
	// Conversion rates (percentages 0–100, 2dp). A zero denominator → 0 (no NaN).
	rep.Conversion = models.FunnelConversion{
		ViewToSubmit:    pct(rep.Totals.Submitted, rep.Totals.Views),
		SubmitToConfirm: pct(rep.Totals.Confirmed, rep.Totals.Submitted),
	}
	return rep, nil
}

// pct returns numerator/denominator × 100 rounded to 2 dp, or 0 if denom == 0.
func pct(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return math.Round(float64(num)/float64(denom)*10000) / 100
}
