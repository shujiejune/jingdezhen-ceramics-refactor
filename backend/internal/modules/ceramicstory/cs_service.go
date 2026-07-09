package ceramicstory

import (
	"context"
	"errors"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines ceramic-story business logic (i18n-aware).
type ServiceInterface interface {
	// --- Public reads ---
	GetAllCeramicStories(ctx context.Context, locale string) ([]models.CeramicStory, error)
	GetCeramicStoryDetail(ctx context.Context, slug string, locale string) (*models.CeramicStory, error)

	// --- Admin / CMS ---
	AdminListStories(ctx context.Context, locale, status string, page, limit int) ([]models.CeramicStory, int, error)
	AdminGetStory(ctx context.Context, slug string, locale string) (*models.CeramicStory, error)
	AdminCreateStory(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error)
	AdminUpdateStory(ctx context.Context, storyID int64, locale string, data models.UpdateCeramicStoryData, actor i18ncontent.WorkflowActor) (*models.CeramicStory, error)
	// AdminTransitionStory applies a content-workflow transition (draft→in_review,
	// in_review→published, etc.) after validating it with i18ncontent.Transition.
	// reviewerID is the acting user (for the reviewed_by column).
	AdminTransitionStory(ctx context.Context, storyID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.CeramicStory, error)
	AdminDeleteStory(ctx context.Context, storyID int64) error
}

type Service struct {
	repo RepositoryInterface
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// GetAllCeramicStories returns all published stories for the given locale.
func (s *Service) GetAllCeramicStories(ctx context.Context, locale string) ([]models.CeramicStory, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
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
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	story, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetCeramicStoryDetail: %w", err)
	}
	return story, nil
}

// --- Admin ---

func (s *Service) AdminListStories(ctx context.Context, locale, status string, page, limit int) ([]models.CeramicStory, int, error) {
	// Admin list: locale/status are optional filters (empty = all). Validate the
	// locale if provided so the CMS doesn't silently show the wrong language.
	if locale != "" {
		var err error
		locale, err = i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			return nil, 0, models.ErrInvalidLocale
		}
	}
	return s.repo.FindAllAdmin(ctx, locale, status, page, limit)
}

func (s *Service) AdminGetStory(ctx context.Context, slug string, locale string) (*models.CeramicStory, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	return s.repo.FindAdminBySlug(ctx, locale, slug)
}

func (s *Service) AdminCreateStory(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error) {
	locale, err := i18ncontent.NormalizeLocale(data.Locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	data.Locale = locale
	return s.repo.CreateWithTranslation(ctx, data)
}

func (s *Service) AdminUpdateStory(ctx context.Context, storyID int64, locale string, data models.UpdateCeramicStoryData, actor i18ncontent.WorkflowActor) (*models.CeramicStory, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	// Gate: a translation may only be edited in draft/rejected (editor) or any
	// state (super admin). Enforced here so a handler bug can't bypass it.
	currentStatus, err := s.repo.GetTranslationStatus(ctx, storyID, locale)
	if err != nil {
		return nil, err
	}
	if !i18ncontent.CanEdit(currentStatus, actor) {
		return nil, models.ErrInvalidWorkflowTransition
	}
	return s.repo.UpdateTranslation(ctx, storyID, locale, data)
}

func (s *Service) AdminTransitionStory(ctx context.Context, storyID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.CeramicStory, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	from, err := s.repo.GetTranslationStatus(ctx, storyID, locale)
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
	if err := s.repo.UpdateTranslationStatus(ctx, storyID, locale, to, reviewer); err != nil {
		return nil, err
	}
	return s.repo.FindAdminByID(ctx, storyID, locale)
}

func (s *Service) AdminDeleteStory(ctx context.Context, storyID int64) error {
	return s.repo.Delete(ctx, storyID)
}

// (unused but keeps the import meaningful if errors.Is is needed later)
var _ = errors.Is
