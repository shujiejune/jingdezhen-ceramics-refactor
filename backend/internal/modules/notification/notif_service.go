package notification

import (
	"context"
	"fmt"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/user"
)

// WebSocketService remains the same.
type WebSocketService interface {
	SendToUser(userID string, notification *models.Notification)
	IsUserOnline(userID string) bool
}

// Service provides business logic for notifications.
type ServiceInterface interface {
	CreateNotification(ctx context.Context, params models.CreateNotificationParams) (*models.Notification, error)
	GetNotificationsForUser(ctx context.Context, userID string, page, limit int) ([]models.Notification, int, error)
	GetUnreadNotificationCount(ctx context.Context, userID string) (int64, error)
	MarkNotificationAsRead(ctx context.Context, notificationID int64, userID string) error
	MarkAllNotificationsAsRead(ctx context.Context, userID string) error
}

type Service struct {
	repo             RepositoryInterface
	userRepo         user.RepositoryInterface
	webSocketService WebSocketService
}

// NewService creates a new notification service.
func NewService(repo RepositoryInterface, userRepo user.RepositoryInterface, wsService WebSocketService) ServiceInterface {
	return &Service{
		repo:             repo,
		userRepo:         userRepo,
		webSocketService: wsService,
	}
}

// CreateNotification creates a new notification, saves it, and pushes it in real-time if the user is online.
func (s *Service) CreateNotification(ctx context.Context, params models.CreateNotificationParams) (*models.Notification, error) {
	message, err := s.generateMessage(params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate notification message: %w", err)
	}

	notification := &models.Notification{
		RecipientUserID:  params.RecipientUserID,
		NotificationType: params.Type,
		Message:          message,
		IsRead:           false,
	}

	// Handle nullable ActorUserID
	if params.ActorUserID != "" {
		notification.ActorUserID = &params.ActorUserID
	}

	// Handle nullable EntityType and EntityID
	if params.EntityType != "" {
		notification.EntityType = &params.EntityType
	}
	if params.EntityID != 0 {
		notification.EntityID = &params.EntityID
	}

	if err := s.repo.Create(ctx, notification); err != nil {
		return nil, err
	}

	// Real-time push logic
	if s.webSocketService != nil && s.webSocketService.IsUserOnline(params.RecipientUserID) {
		// Before sending, populate the ActorUser field here for the real-time message.
		actor, err := s.userRepo.FindByID(ctx, params.ActorUserID)
		if err != nil {
			log.Printf("WARN: could not find actor user %s for real-time push: %v", params.ActorUserID, err)
		} else {
			notification.ActorUser = actor
		}
		s.webSocketService.SendToUser(params.RecipientUserID, notification)
	}

	return notification, nil
}

// GetNotificationsForUser retrieves notifications and composes them with actor details.
func (s *Service) GetNotificationsForUser(ctx context.Context, userID string, page, limit int) ([]models.Notification, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// 1. Get total count for pagination
	total, err := s.repo.GetTotalCountByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get the raw notifications from the database
	notifications, err := s.repo.GetByUserID(ctx, userID, limit, offset)
	if err != nil || len(notifications) == 0 {
		return notifications, total, err
	}

	// 3. Collect all unique, non-nil actor IDs
	actorIDsSet := make(map[string]struct{})
	for _, n := range notifications {
		if n.ActorUserID != nil {
			actorIDsSet[*n.ActorUserID] = struct{}{}
		}
	}

	// 4. If there are actors to fetch, fetch them all in one query.
	if len(actorIDsSet) > 0 {
		// Convert the set (map keys) to a slice.
		actorIDsSlice := make([]string, 0, len(actorIDsSet))
		for id := range actorIDsSet {
			actorIDsSlice = append(actorIDsSlice, id)
		}

		// Fetch all users in a single database call.
		actors, err := s.userRepo.FindByIDs(ctx, actorIDsSlice)
		if err != nil {
			// Handle gracefully: Log the error but return the notifications anyway.
			log.Printf("ERROR: could not fetch actor users for notifications: %v", err)
		} else {
			// Create a map for quick lookups.
			actorsByID := make(map[string]*models.User, len(actors))
			for i := range actors {
				actorsByID[actors[i].ID] = &actors[i]
			}

			// 5. Stitch the actor data onto the notifications.
			for i := range notifications {
				if notifications[i].ActorUserID != nil {
					if actor, ok := actorsByID[*notifications[i].ActorUserID]; ok {
						notifications[i].ActorUser = actor
					}
				}
			}
		}
	}

	return notifications, total, nil
}

func (s *Service) GetUnreadNotificationCount(ctx context.Context, userID string) (int64, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

func (s *Service) MarkNotificationAsRead(ctx context.Context, notificationID int64, userID string) error {
	updated, err := s.repo.MarkAsRead(ctx, notificationID, userID)
	if err != nil {
		return err
	}
	if !updated {
		return models.ErrNotFound
	}
	return nil
}

func (s *Service) MarkAllNotificationsAsRead(ctx context.Context, userID string) error {
	_, err := s.repo.MarkAllAsRead(ctx, userID)
	return err
}

// generateMessage constructs the human-readable notification message.
func (s *Service) generateMessage(params models.CreateNotificationParams) (string, error) {
	// NOTE: This function would use the userRepo to get the actor's name.
	actorName := params.ExtraData["actorName"]
	if actorName == "" {
		actorName = "Someone"
	}

	_ = actorName // reserved for actor-based notification types (e.g. chat handoff)

	switch params.Type {
	case models.NotificationTypeSystem:
		return params.ExtraData["message"], nil
	case models.NotificationTypeLowStock:
		// PRD §3.4.1: low-stock alert to E-commerce Operators.
		return params.ExtraData["message"], nil
	// PRD-aligned types (order status, itinerary status, chat handoff,
	// content approval) will be added as their modules land.
	default:
		return "", fmt.Errorf("unrecognized notification type: %s", params.Type)
	}
}
