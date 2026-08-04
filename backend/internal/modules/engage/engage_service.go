package engage

import (
	"context"
	"fmt"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines Destinations & Local Lifestyle business logic (i18n-aware).
type ServiceInterface interface {
	// --- Public reads ---
	GetActivities(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error)
	GetActivityArticle(ctx context.Context, slug string, locale string) (*models.Activity, error)

	// --- Admin / CMS ---
	AdminListActivities(ctx context.Context, locale, status, typeFilter string, page, limit int) ([]models.Activity, int, error)
	AdminGetActivity(ctx context.Context, slug string, locale string) (*models.Activity, error)
	AdminCreateActivity(ctx context.Context, data models.CreateActivityData) (*models.Activity, error)
	AdminUpdateActivity(ctx context.Context, activityID int64, locale string, data models.UpdateActivityData, actor i18ncontent.WorkflowActor) (*models.Activity, error)
	AdminTransitionActivity(ctx context.Context, activityID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Activity, error)
	AdminDeleteActivity(ctx context.Context, activityID int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// --- Public ---

func (s *Service) GetActivities(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	activities, total, err := s.repo.FindAllPublished(ctx, locale, typeFilter, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetActivities: %w", err)
	}
	return activities, total, nil
}

func (s *Service) GetActivityArticle(ctx context.Context, slug string, locale string) (*models.Activity, error) {
	if slug == "" {
		return nil, fmt.Errorf("service.GetActivityArticle: slug cannot be empty")
	}
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	article, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetActivityArticle: %w", err)
	}
	return article, nil
}

// --- Admin ---

func (s *Service) AdminListActivities(ctx context.Context, locale, status, typeFilter string, page, limit int) ([]models.Activity, int, error) {
	if locale != "" {
		var err error
		locale, err = i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			return nil, 0, models.ErrInvalidLocale
		}
	}
	return s.repo.FindAllAdmin(ctx, locale, status, typeFilter, page, limit)
}

func (s *Service) AdminGetActivity(ctx context.Context, slug string, locale string) (*models.Activity, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	return s.repo.FindAdminBySlug(ctx, locale, slug)
}

func (s *Service) AdminCreateActivity(ctx context.Context, data models.CreateActivityData) (*models.Activity, error) {
	locale, err := i18ncontent.NormalizeLocale(data.Locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	data.Locale = locale
	return s.repo.CreateWithTranslation(ctx, data)
}

func (s *Service) AdminUpdateActivity(ctx context.Context, activityID int64, locale string, data models.UpdateActivityData, actor i18ncontent.WorkflowActor) (*models.Activity, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	currentStatus, err := s.repo.GetTranslationStatus(ctx, activityID, locale)
	if err != nil {
		return nil, err
	}
	if !i18ncontent.CanEdit(currentStatus, actor) {
		return nil, models.ErrInvalidWorkflowTransition
	}
	return s.repo.UpdateTranslation(ctx, activityID, locale, data)
}

func (s *Service) AdminTransitionActivity(ctx context.Context, activityID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Activity, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	from, err := s.repo.GetTranslationStatus(ctx, activityID, locale)
	if err != nil {
		return nil, err
	}
	if _, err := i18ncontent.Transition(from, to, actor); err != nil {
		return nil, err
	}
	var reviewer *string
	if reviewerID != "" {
		reviewer = &reviewerID
	}
	if err := s.repo.UpdateTranslationStatus(ctx, activityID, locale, to, reviewer); err != nil {
		return nil, err
	}
	return s.repo.FindAdminByID(ctx, activityID, locale)
}

func (s *Service) AdminDeleteActivity(ctx context.Context, activityID int64) error {
	return s.repo.Delete(ctx, activityID)
}
