package engage

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
)

type ServiceInterface interface {
	GetActivities(ctx context.Context, page, limit int) ([]models.Activity, int, error)
	GetActivityArticle(ctx context.Context, idOrSlug string) (*models.Article, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// GetActivities retrieves a paginated list of activities.
func (s *Service) GetActivities(ctx context.Context, page, limit int) ([]models.Activity, int, error) {
	// Business logic could be added here in the future, e.g., filtering out past events.
	activities, total, err := s.repo.FindAllActivities(ctx, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetActivities: %w", err)
	}
	return activities, total, nil
}

// GetActivityArticle retrieves the detailed article for an activity.
func (s *Service) GetActivityArticle(ctx context.Context, idOrSlug string) (*models.Article, error) {
	if idOrSlug == "" {
		return nil, errors.New("service.GetActivityArticle: idOrSlug cannot be empty")
	}
	article, err := s.repo.FindArticleByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, fmt.Errorf("service.GetActivityArticle: %w", err)
	}
	// Business logic could be added here, e.g., incrementing a view count.
	return article, nil
}
