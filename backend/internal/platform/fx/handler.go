package fx

import (
	"context"
	"errors"
	"log"

	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
)

// JobEnqueuer is the subset of the Asynq client needed to trigger an FX refresh.
// (Same pattern as user.EmailEnqueuer — keeps the FX handler decoupled from jobs.)
type JobEnqueuer interface {
	EnqueueFXRefresh(ctx context.Context) error
}

// Handler exposes FX admin + debug endpoints (TDD §7, PRD §3.2.3).
type Handler struct {
	service     ServiceInterface
	jobEnqueuer JobEnqueuer
}

func NewHandler(service ServiceInterface, jobEnqueuer JobEnqueuer) *Handler {
	return &Handler{service: service, jobEnqueuer: jobEnqueuer}
}

// RefreshFX: POST /admin/fx/refresh — enqueue the fx:refresh job (worker fetches
// ECB rates, applies markup, upserts). Guarded by PermSettingsManage.
//
// @Summary      Enqueue an FX rate refresh
// @Description  Enqueues the fx:refresh job (worker fetches ECB rates, applies markup, upserts).
// @Description  Access: super_admin (settings.manage).
// @Tags         admin,fx
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <access_token>"
// @Success      202 {object} object "{message: \"FX refresh enqueued\"}"
// @Failure      401 {object} models.ErrorResponse "Authentication required"
// @Failure      403 {object} models.ErrorResponse "Forbidden (needs settings.manage)"
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Failure      503 {object} models.ErrorResponse "FX refresh job queue not configured"
// @Security     BearerAuth
// @Router       /admin/fx/refresh [post]
func (h *Handler) RefreshFX(c *fiber.Ctx) error {
	if h.jobEnqueuer == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(models.ErrorResponse{
			Message: "FX refresh job queue not configured",
		})
	}
	if err := h.jobEnqueuer.EnqueueFXRefresh(c.Context()); err != nil {
		log.Printf("Handler.RefreshFX: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Failed to enqueue FX refresh",
		})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"message": "FX refresh enqueued",
	})
}

// ListRates: GET /fx/rates — dev-debug: current fx_rates rows.
//
// @Summary      List current FX rates (dev-debug)
// @Description  Returns the current fx_rates rows (dev-debug endpoint).
// @Tags         fx
// @Accept       json
// @Produce      json
// @Success      200 {array} object
// @Failure      500 {object} models.ErrorResponse "Internal error"
// @Failure      503 {object} models.ErrorResponse "FX rate listing not available"
// @Router       /fx/rates [get]
func (h *Handler) ListRates(c *fiber.Ctx) error {
	repo, ok := h.repoForList()
	if !ok {
		return c.Status(fiber.StatusServiceUnavailable).JSON(models.ErrorResponse{
			Message: "FX rate listing not available",
		})
	}
	rows, err := repo.ListAll(c.Context())
	if err != nil {
		log.Printf("Handler.ListRates: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Failed to retrieve FX rates",
		})
	}
	return c.Status(fiber.StatusOK).JSON(rows)
}

// repoForList extracts the repository from the service for the debug endpoint.
// The service holds it privately; for the debug list we re-assert via a cast to
// the concrete *Service (which embeds repo). This keeps the debug path from
// leaking a repo field into ServiceInterface.
func (h *Handler) repoForList() (RepositoryInterface, bool) {
	if s, ok := h.service.(*Service); ok {
		return s.repo, true
	}
	return nil, false
}

// (compile-time guards)
var _ = errors.Is
