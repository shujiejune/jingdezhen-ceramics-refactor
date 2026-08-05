package analytics

import (
	"encoding/csv"
	"errors"
	"log"
	"strconv"
	"time"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
)

// maxRangeDays caps the dashboard date range (PRD §3.4.2 custom-range filter).
// 366 days keeps the response payload bounded + the zero-filled series sane
// while allowing the `year` preset (which spans 365 raw days + up to 1 day
// from the inclusive-`to` normalization, i.e. up to 366).
const maxRangeDays = 366

// errRangeInvalid is returned by parseRange for an unparseable/inverted/oversize
// range; the handler maps it to 400. Kept package-local (not a domain error in
// models/errors.go) since it only surfaces as an HTTP 400 from these handlers.
var errRangeInvalid = errors.New("invalid date range")

// parseRange resolves the dashboard date-range filter (PRD §3.4.2). Supports:
//   - ?range=day|week|month|quarter|year (server computes [now-start, now))
//   - ?from=YYYY-MM-DD&to=YYYY-MM-DD  (explicit; from inclusive, to exclusive)
//
// Defaults to the last 30 days. The window is returned as UTC start-of-day
// `from` (inclusive) and exclusive `to` (start-of-day of to+1, or now if `to`
// is omitted under a preset). Returns ErrValidation on from>to or >365-day span.
func parseRange(c *fiber.Ctx) (from, to time.Time, err error) {
	now := time.Now().UTC()
	preset := c.Query("range")
	fromStr := c.Query("from")
	toStr := c.Query("to")

	switch {
	case preset != "":
		from, to, err = presetRange(preset, now)
	case fromStr != "" || toStr != "":
		from, to, err = explicitRange(fromStr, toStr, now)
	default:
		// Last 30 days.
		to = now
		from = to.AddDate(0, 0, -30)
	}
	if err != nil {
		return from, to, err
	}

	// Normalize to UTC day boundaries (inclusive from, exclusive to next day).
	from = from.UTC().Truncate(24 * time.Hour)
	if to.IsZero() {
		to = now
	}
	toDay := to.UTC().Truncate(24 * time.Hour)
	if toDay.Equal(to.UTC()) {
		// `to` landed exactly on a day boundary → treat as exclusive start of that day.
		// (The caller passed a bare date, not now.)
	} else {
		toDay = toDay.AddDate(0, 0, 1) // include the whole `to` day
	}
	to = toDay

	if !to.After(from) {
		return from, to, errRangeInvalid
	}
	if days := int(to.Sub(from) / (24 * time.Hour)); days > maxRangeDays {
		return from, to, errRangeInvalid
	}
	return from, to, nil
}

// presetRange computes [now-start, now) for a named preset.
func presetRange(p string, now time.Time) (time.Time, time.Time, error) {
	to := now
	var from time.Time
	switch p {
	case "day":
		from = to.AddDate(0, 0, -1)
	case "week":
		from = to.AddDate(0, 0, -7)
	case "month":
		from = to.AddDate(0, -1, 0)
	case "quarter":
		from = to.AddDate(0, -3, 0)
	case "year":
		from = to.AddDate(-1, 0, 0)
	default:
		return from, to, errRangeInvalid
	}
	return from, to, nil
}

// explicitRange parses YYYY-MM-DD from/to. `to` optional → now.
func explicitRange(fromStr, toStr string, now time.Time) (time.Time, time.Time, error) {
	from, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC)
	if err != nil {
		return from, now, errRangeInvalid
	}
	var to time.Time
	if toStr == "" {
		to = now
	} else {
		t, err := time.ParseInLocation("2006-01-02", toStr, time.UTC)
		if err != nil {
			return from, now, errRangeInvalid
		}
		to = t.AddDate(0, 0, 1) // include the whole to-day (make it exclusive)
	}
	return from, to, nil
}

// --- Dashboard handler -------------------------------------------------------

// DashboardHandler serves the admin dashboard read endpoints. It is a distinct
// handler from the public RecordEvent one so the wiring stays clear (ingest vs
// dashboard). Constructed with the DashboardServiceInterface.
type DashboardHandler struct {
	service DashboardServiceInterface
}

func NewDashboardHandler(service DashboardServiceInterface) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Traffic: GET /admin/analytics/traffic — traffic report (PRD §3.4.2).
//
// @Summary      Traffic dashboard report
// @Description  Traffic analysis: page views, visitors (by GeoIP country), top content, by locale.
// @Description  Reads live from analytics_events (consent-gated at ingest). Range filter via
// @Description  ?range=day|week|month|quarter|year or ?from=&to= (YYYY-MM-DD, default 30 days, max 365).
// @Description  ?format=csv streams a flattened CSV (Content-Disposition: attachment).
// @Tags         admin,analytics
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        range query string false "day|week|month|quarter|year (server computes [now-start, now))"
// @Param        from query string false "YYYY-MM-DD (inclusive; requires to)"
// @Param        to query string false "YYYY-MM-DD (inclusive)"
// @Param        format query string false "csv = stream a flattened CSV report"
// @Success      200 {object} models.TrafficReport
// @Failure      400 {object} models.ErrorResponse "Invalid date range"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs dashboard.view)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/analytics/traffic [get]
func (h *DashboardHandler) Traffic(c *fiber.Ctx) error {
	from, to, err := parseRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid date range"})
	}
	rep, err := h.service.TrafficReport(c.Context(), from, to)
	if err != nil {
		log.Printf("Handler.Traffic: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to load traffic report"})
	}
	if c.Query("format") == "csv" {
		return writeTrafficCSV(c, rep)
	}
	return c.JSON(rep)
}

// Sales: GET /admin/analytics/sales — sales/GMV dashboard report (PRD §3.4.2).
//
// @Summary      Sales dashboard report
// @Description  Sales analysis: GMV (Σ subtotal_cny, excludes shipping) over realized orders
// @Description  (status IN paid|shipped|completed — cancelled/refunded excluded), by
// @Description  currency/region/product/artist, time series. Range filter as for /traffic.
// @Description  ?format=csv streams a flattened CSV (Content-Disposition: attachment).
// @Tags         admin,analytics
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        range query string false "day|week|month|quarter|year"
// @Param        from query string false "YYYY-MM-DD (inclusive)"
// @Param        to query string false "YYYY-MM-DD (inclusive)"
// @Param        format query string false "csv"
// @Success      200 {object} models.SalesReport
// @Failure      400 {object} models.ErrorResponse "Invalid date range"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs dashboard.view)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/analytics/sales [get]
func (h *DashboardHandler) Sales(c *fiber.Ctx) error {
	from, to, err := parseRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid date range"})
	}
	rep, err := h.service.SalesReport(c.Context(), from, to)
	if err != nil {
		log.Printf("Handler.Sales: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to load sales report"})
	}
	if c.Query("format") == "csv" {
		return writeSalesCSV(c, rep)
	}
	return c.JSON(rep)
}

// Funnel: GET /admin/analytics/funnel — itinerary conversion funnel (PRD §3.4.2).
//
// @Summary      Itinerary conversion funnel
// @Description  Itinerary conversion funnel: form views (analytics_events name=
// @Description  itinerary_form_view) → submissions (itinerary_requests submitted_at in range)
// @Description  → confirmed (cohort status='confirmed'), conversion rates over time. Cohort
// @Description  semantics (no confirmed_at column). Range filter as for /traffic.
// @Description  ?format=csv streams a flattened CSV (Content-Disposition: attachment).
// @Tags         admin,analytics
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        range query string false "day|week|month|quarter|year"
// @Param        from query string false "YYYY-MM-DD (inclusive)"
// @Param        to query string false "YYYY-MM-DD (inclusive)"
// @Param        format query string false "csv"
// @Success      200 {object} models.FunnelReport
// @Failure      400 {object} models.ErrorResponse "Invalid date range"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs dashboard.view)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/analytics/funnel [get]
func (h *DashboardHandler) Funnel(c *fiber.Ctx) error {
	from, to, err := parseRange(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid date range"})
	}
	rep, err := h.service.FunnelReport(c.Context(), from, to)
	if err != nil {
		log.Printf("Handler.Funnel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to load funnel report"})
	}
	if c.Query("format") == "csv" {
		return writeFunnelCSV(c, rep)
	}
	return c.JSON(rep)
}

// --- CSV writers (flatten each report to a sensible table) -------------------

func writeTrafficCSV(c *fiber.Ctx, r *models.TrafficReport) error {
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="analytics-traffic.csv"`)
	w := csv.NewWriter(c.Response().BodyWriter())
	if err := w.Write([]string{"date", "pageviews", "visitors", "events"}); err != nil {
		return err
	}
	for _, p := range r.Series {
		if err := w.Write([]string{p.Date, strconv.FormatInt(p.Pageviews, 10),
			strconv.FormatInt(p.Visitors, 10), strconv.FormatInt(p.Events, 10)}); err != nil {
			return err
		}
	}
	w.Flush()
	return nil
}

func writeSalesCSV(c *fiber.Ctx, r *models.SalesReport) error {
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="analytics-sales.csv"`)
	w := csv.NewWriter(c.Response().BodyWriter())
	if err := w.Write([]string{"date", "gmv_cny", "orders"}); err != nil {
		return err
	}
	for _, p := range r.Series {
		if err := w.Write([]string{p.Date, strconv.FormatInt(p.GMVCny, 10),
			strconv.FormatInt(p.Orders, 10)}); err != nil {
			return err
		}
	}
	w.Flush()
	return nil
}

func writeFunnelCSV(c *fiber.Ctx, r *models.FunnelReport) error {
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="analytics-funnel.csv"`)
	w := csv.NewWriter(c.Response().BodyWriter())
	if err := w.Write([]string{"date", "views", "submitted", "confirmed"}); err != nil {
		return err
	}
	for _, p := range r.Series {
		if err := w.Write([]string{p.Date, strconv.FormatInt(p.Views, 10),
			strconv.FormatInt(p.Submitted, 10), strconv.FormatInt(p.Confirmed, 10)}); err != nil {
			return err
		}
	}
	w.Flush()
	return nil
}
