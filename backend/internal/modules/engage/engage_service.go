package engage

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines Destinations & Local Lifestyle business logic (i18n-aware).
type ServiceInterface interface {
	GetActivities(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error)
	GetActivityArticle(ctx context.Context, slug string, locale string) (*models.Activity, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// GetActivities returns published activities for a locale, optionally filtered by
// the parent `type` (e.g. "Destination" vs "Local Lifestyle"), paginated.
func (s *Service) GetActivities(ctx context.Context, locale, typeFilter string, page, limit int) ([]models.Activity, int, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, false)
	if err != nil {
		locale = models.DefaultLocale
	}
	activities, total, err := s.repo.FindAllPublished(ctx, locale, typeFilter, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetActivities: %w", err)
	}
	return activities, total, nil
}

// GetActivityArticle returns a single published activity by (slug, locale).
func (s *Service) GetActivityArticle(ctx context.Context, slug string, locale string) (*models.Activity, error) {
	if slug == "" {
		return nil, fmt.Errorf("service.GetActivityArticle: slug cannot be empty")
	}
	locale, err := i18ncontent.NormalizeLocale(locale, false)
	if err != nil {
		locale = models.DefaultLocale
	}
	article, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetActivityArticle: %w", err)
	}
	return article, nil
}
