package audit

import (
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
)

// Helper wraps a Logger with Fiber-context actor extraction, so each sensitive
// handler only needs one field + one call (no per-handler boilerplate for
// GetUserIDFromContext + c.IP()). Nil-safe: a nil Helper is a no-op.
type Helper struct {
	logger Logger
}

// NewHelper wraps a Logger. Pass nil to get a no-op Helper (tests/worker).
func NewHelper(l Logger) *Helper {
	if l == nil {
		return nil
	}
	return &Helper{logger: l}
}

// Log records one audit entry, extracting the actor ID + IP from the Fiber
// context. Best-effort: a failure is logged but never blocks the action (the
// action already succeeded by the time Log is called).
func (h *Helper) Log(c *fiber.Ctx, action models.AuditAction, entityType models.AuditEntityType,
	entityID string, detail map[string]any) {
	if h == nil || h.logger == nil {
		return
	}
	actorID, _ := utils.GetUserIDFromContext(c)
	if err := h.logger.Log(c.Context(), actorID, c.IP(), action, entityType, entityID, detail); err != nil {
		log.Printf("audit.Log(%s %s/%s): %v", action, entityType, entityID, err)
	}
}

// ActionForTransition maps a content-workflow target status to the audit action
// (approve/reject/unpublish). Shared by the 4 content modules' adminTransition.
func ActionForTransition(to models.ContentStatus) models.AuditAction {
	switch to {
	case models.StatusPublished:
		return models.AuditActionContentApprove
	case models.StatusRejected:
		return models.AuditActionContentReject
	case models.StatusDraft:
		return models.AuditActionContentUnpublish
	default:
		return "" // submit/in_review not audited (not a publish/delete)
	}
}
