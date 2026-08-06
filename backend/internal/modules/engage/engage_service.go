package engage

import (
	"context"
	"fmt"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/sitemap"
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
	// SetSitemapEnqueuer wires the sitemap-rebuild trigger (PRD §4.4).
	SetSitemapEnqueuer(e sitemap.Enqueuer)
	// SetGalleryLoader wires the ordered-media-gallery loader (PRD §3.1.2/§3.1.3).
	SetGalleryLoader(gl GalleryLoader)
}

type Service struct {
	repo            RepositoryInterface
	sitemapEnqueuer sitemap.Enqueuer // optional; nil => no sitemap rebuild (worker/tests)
	galleryLoader   GalleryLoader    // optional; nil => no gallery (list view/tests)
}

// GalleryLoader loads an activity's ordered media gallery. Implemented by the
// media service; injected post-construction to avoid an engage→media import edge.
type GalleryLoader interface {
	ListActivityMedia(ctx context.Context, activityID int64) ([]models.GalleryItem, error)
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// SetSitemapEnqueuer wires the sitemap-rebuild trigger (PRD §4.4). nil-safe.
func (s *Service) SetSitemapEnqueuer(e sitemap.Enqueuer) { s.sitemapEnqueuer = e }

// SetGalleryLoader wires the gallery loader (PRD §3.1.2/§3.1.3). nil-safe.
func (s *Service) SetGalleryLoader(gl GalleryLoader) { s.galleryLoader = gl }

// enqueueSitemapRebuild fires a sitemap rebuild best-effort (PRD §4.4). nil-
// safe (worker/tests); logs on error, never returns one.
func (s *Service) enqueueSitemapRebuild(ctx context.Context) {
	if s.sitemapEnqueuer == nil {
		return
	}
	if err := s.sitemapEnqueuer.EnqueueSitemapRebuild(ctx); err != nil {
		log.Printf("engage.enqueueSitemapRebuild: %v", err)
	}
}

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
	// hreflang alternates (PRD §4.4): locale→slug for every OTHER published
	// translation. Best-effort — a failure logs + leaves Alternates empty.
	alts, aerr := s.repo.FindPublishedAlternates(ctx, article.ID, locale)
	if aerr != nil {
		log.Printf("engage.GetActivityArticle.Alternates(%d): %v", article.ID, aerr)
	} else if len(alts) > 0 {
		article.Alternates = alts
	}
	// Load the ordered media gallery (detail view).
	if s.galleryLoader != nil {
		gallery, gerr := s.galleryLoader.ListActivityMedia(ctx, article.ID)
		if gerr != nil {
			log.Printf("engage.GetActivityArticle.Gallery(%d): %v", article.ID, gerr)
		} else if len(gallery) > 0 {
			article.Gallery = gallery
		}
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
	if to == models.StatusPublished || from == models.StatusPublished {
		s.enqueueSitemapRebuild(ctx)
	}
	return s.repo.FindAdminByID(ctx, activityID, locale)
}

func (s *Service) AdminDeleteActivity(ctx context.Context, activityID int64) error {
	return s.repo.Delete(ctx, activityID)
}
