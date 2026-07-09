package ceramicstory

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines ceramic-story business logic (i18n-aware).
type ServiceInterface interface {
	GetAllCeramicStories(ctx context.Context, locale string) ([]models.CeramicStory, error)
	GetCeramicStoryDetail(ctx context.Context, slug string, locale string) (*models.CeramicStory, error)
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// GetAllCeramicStories returns all published stories for the given locale.
// An empty locale falls back to the default (en-US); an unsupported locale also
// falls back (public reads never 404 on a bad locale, per i18ncontent).
func (s *Service) GetAllCeramicStories(ctx context.Context, locale string) ([]models.CeramicStory, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, false)
	if err != nil {
		// validate=false never returns an error, but guard for future changes.
		locale = models.DefaultLocale
	}
	stories, err := s.repo.FindAllPublished(ctx, locale)
	if err != nil {
		return nil, fmt.Errorf("service.GetAllCeramicStories: %w", err)
	}
	return stories, nil
}

// GetCeramicStoryDetail returns a single published story by (slug, locale).
func (s *Service) GetCeramicStoryDetail(ctx context.Context, slug string, locale string) (*models.CeramicStory, error) {
	if slug == "" {
		return nil, fmt.Errorf("service.GetCeramicStoryDetail: slug cannot be empty")
	}
	locale, err := i18ncontent.NormalizeLocale(locale, false)
	if err != nil {
		locale = models.DefaultLocale
	}
	story, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetCeramicStoryDetail: %w", err)
	}
	return story, nil
}
