package analytics

import (
	"encoding/csv"
	"log"
	"strconv"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

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
//	@Summary		Traffic dashboard report
//	@Description	Traffic analysis: page views, visitors (by GeoIP country), top content, by locale.
//	@Description	Reads live from analytics_events (consent-gated at ingest). Range filter via
//	@Description	?range=day|week|month|quarter|year or ?from=&to= (YYYY-MM-DD, default 30 days, max 365).
//	@Description	?format=csv streams a flattened CSV (Content-Disposition: attachment).
//	@Tags			admin,analytics
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			range			query		string	false	"day|week|month|quarter|year (server computes [now-start, now))"
//	@Param			from			query		string	false	"YYYY-MM-DD (inclusive; requires to)"
//	@Param			to				query		string	false	"YYYY-MM-DD (inclusive)"
//	@Param			format			query		string	false	"csv = stream a flattened CSV report"
//	@Success		200				{object}	models.TrafficReport
//	@Failure		400				{object}	models.ErrorResponse	"Invalid date range"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs dashboard.view)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/analytics/traffic [get]
func (h *DashboardHandler) Traffic(c *fiber.Ctx) error {
	from, to, err := utils.ParseRange(c)
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
//	@Summary		Sales dashboard report
//	@Description	Sales analysis: GMV (Σ subtotal_cny, excludes shipping) over realized orders
//	@Description	(status IN paid|shipped|completed — cancelled/refunded excluded), by
//	@Description	currency/region/product/artist, time series. Range filter as for /traffic.
//	@Description	?format=csv streams a flattened CSV (Content-Disposition: attachment).
//	@Tags			admin,analytics
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			range			query		string	false	"day|week|month|quarter|year"
//	@Param			from			query		string	false	"YYYY-MM-DD (inclusive)"
//	@Param			to				query		string	false	"YYYY-MM-DD (inclusive)"
//	@Param			format			query		string	false	"csv"
//	@Success		200				{object}	models.SalesReport
//	@Failure		400				{object}	models.ErrorResponse	"Invalid date range"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs dashboard.view)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/analytics/sales [get]
func (h *DashboardHandler) Sales(c *fiber.Ctx) error {
	from, to, err := utils.ParseRange(c)
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
//	@Summary		Itinerary conversion funnel
//	@Description	Itinerary conversion funnel: form views (analytics_events name=
//	@Description	itinerary_form_view) → submissions (itinerary_requests submitted_at in range)
//	@Description	→ confirmed (cohort status='confirmed'), conversion rates over time. Cohort
//	@Description	semantics (no confirmed_at column). Range filter as for /traffic.
//	@Description	?format=csv streams a flattened CSV (Content-Disposition: attachment).
//	@Tags			admin,analytics
//	@Produce		json
//	@Param			Authorization	header		string	true	"Bearer <access_token>"
//	@Param			range			query		string	false	"day|week|month|quarter|year"
//	@Param			from			query		string	false	"YYYY-MM-DD (inclusive)"
//	@Param			to				query		string	false	"YYYY-MM-DD (inclusive)"
//	@Param			format			query		string	false	"csv"
//	@Success		200				{object}	models.FunnelReport
//	@Failure		400				{object}	models.ErrorResponse	"Invalid date range"
//	@Failure		401				{object}	models.ErrorResponse	"Authentication required"
//	@Failure		403				{object}	models.ErrorResponse	"Forbidden (needs dashboard.view)"
//	@Failure		500				{object}	models.ErrorResponse	"Internal error"
//	@Security		BearerAuth
//	@Router			/admin/analytics/funnel [get]
func (h *DashboardHandler) Funnel(c *fiber.Ctx) error {
	from, to, err := utils.ParseRange(c)
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
