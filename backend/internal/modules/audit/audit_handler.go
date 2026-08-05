package audit

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service ServiceInterface
}

func NewHandler(service ServiceInterface) *Handler {
	return &Handler{service: service}
}

// List: GET /admin/audit-log — the accountability trail (PRD §3.1.1).
//
// @Summary      List audit log entries
// @Description  Returns the admin audit log (sensitive actions) with filters + pagination.
// @Description  Access: super_admin (settings.manage). Supports ?format=csv for export.
// @Tags         admin,audit
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Param        actor_id query string false "Filter by actor (UUID)"
// @Param        action query string false "Filter by action (e.g. order.refund)"
// @Param        entity_type query string false "Filter by entity type (e.g. order)"
// @Param        entity_id query string false "Filter by entity ID"
// @Param        range query string false "day|week|month|quarter|year"
// @Param        from query string false "YYYY-MM-DD (inclusive)"
// @Param        to query string false "YYYY-MM-DD (inclusive)"
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Page size (default 20, max 100)"
// @Param        format query string false "csv = stream a flattened CSV"
// @Success      200 {object} models.PaginatedResponse "data: []models.AuditLog"
// @Failure      400 {object} models.ErrorResponse "Invalid date range"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs settings.manage)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Security     BearerAuth
// @Router       /admin/audit-log [get]
func (h *Handler) List(c *fiber.Ctx) error {
	page, limit := utils.GetPageLimit(c)

	f := models.AuditLogFilter{
		Action:     models.AuditAction(c.Query("action")),
		EntityType: models.AuditEntityType(c.Query("entity_type")),
	}
	if aid := c.Query("actor_id"); aid != "" {
		f.ActorID = &aid
	}
	if eid := c.Query("entity_id"); eid != "" {
		f.EntityID = &eid
	}
	// Date range is optional; only applied if from/to or range present.
	if c.Query("from") != "" || c.Query("to") != "" || c.Query("range") != "" {
		from, to, err := utils.ParseRange(c)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Invalid date range"})
		}
		f.From = from
		f.To = to
	}

	rows, total, err := h.service.List(c.Context(), f, page, limit)
	if err != nil {
		log.Printf("Handler.AuditList: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{Message: "Failed to load audit log"})
	}

	if c.Query("format") == "csv" {
		return writeAuditCSV(c, rows)
	}
	return c.Status(fiber.StatusOK).JSON(models.NewPaginatedResponse(rows, page, limit, total))
}

// writeAuditCSV streams the audit rows as CSV (reuses the itinerary/analytics
// export pattern). detail is flattened to a JSON string column.
func writeAuditCSV(c *fiber.Ctx, rows []models.AuditLog) error {
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
	w := csv.NewWriter(c.Response().BodyWriter())
	if err := w.Write([]string{"id", "created_at", "actor_id", "action", "entity_type", "entity_id", "detail"}); err != nil {
		return err
	}
	for _, r := range rows {
		actor := ""
		if r.ActorID != nil {
			actor = *r.ActorID
		}
		eid := ""
		if r.EntityID != nil {
			eid = *r.EntityID
		}
		detail := "{}"
		if r.Detail != nil {
			if b, err := json.Marshal(r.Detail); err == nil {
				detail = strings.TrimSpace(string(b))
			}
		}
		if err := w.Write([]string{
			strconv.FormatInt(r.ID, 10),
			r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			actor,
			string(r.Action),
			string(r.EntityType),
			eid,
			detail,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return nil
}
