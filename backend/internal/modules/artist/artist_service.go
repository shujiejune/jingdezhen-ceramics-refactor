package artist

import (
	"context"
	"fmt"
	"log"

	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/sitemap"
	"jingdezhen-ceramics-backend/internal/platform/i18ncontent"
)

// ServiceInterface defines artist-profile business logic (i18n-aware).
type ServiceInterface interface {
	// --- Public reads ---
	GetArtists(ctx context.Context, locale string, page, limit int) ([]models.Artist, int, error)
	GetArtistBySlug(ctx context.Context, slug string, locale string) (*models.Artist, error)

	// --- Admin / CMS ---
	AdminListArtists(ctx context.Context, locale, status string, page, limit int) ([]models.Artist, int, error)
	AdminGetArtist(ctx context.Context, slug string, locale string) (*models.Artist, error)
	AdminCreateArtist(ctx context.Context, data models.CreateArtistData) (*models.Artist, error)
	AdminUpdateArtist(ctx context.Context, artistID int64, locale string, data models.UpdateArtistData, actor i18ncontent.WorkflowActor) (*models.Artist, error)
	AdminTransitionArtist(ctx context.Context, artistID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Artist, error)
	AdminDeleteArtist(ctx context.Context, artistID int64) error
	// SetSitemapEnqueuer wires the sitemap-rebuild trigger (PRD §4.4).
	SetSitemapEnqueuer(e sitemap.Enqueuer)
	// SetGalleryLoader wires the ordered-media-gallery loader (PRD §3.1.3).
	SetGalleryLoader(gl GalleryLoader)
}

type Service struct {
	repo            RepositoryInterface
	sitemapEnqueuer sitemap.Enqueuer // optional; nil => no sitemap rebuild (worker/tests)
	galleryLoader   GalleryLoader    // optional; nil => no gallery (list view/tests)
}

// GalleryLoader loads an artist's ordered media gallery. Implemented by the
// media service; injected post-construction to avoid an artist→media import edge.
type GalleryLoader interface {
	ListArtistMedia(ctx context.Context, artistID int64) ([]models.GalleryItem, error)
}

func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// SetSitemapEnqueuer wires the sitemap-rebuild trigger (PRD §4.4). nil-safe.
func (s *Service) SetSitemapEnqueuer(e sitemap.Enqueuer) { s.sitemapEnqueuer = e }

// SetGalleryLoader wires the gallery loader (PRD §3.1.3). nil-safe.
func (s *Service) SetGalleryLoader(gl GalleryLoader) { s.galleryLoader = gl }

// enqueueSitemapRebuild fires a sitemap rebuild best-effort (PRD §4.4). nil-
// safe (worker/tests); logs on error, never returns one.
func (s *Service) enqueueSitemapRebuild(ctx context.Context) {
	if s.sitemapEnqueuer == nil {
		return
	}
	if err := s.sitemapEnqueuer.EnqueueSitemapRebuild(ctx); err != nil {
		log.Printf("artist.enqueueSitemapRebuild: %v", err)
	}
}

func (s *Service) GetArtists(ctx context.Context, locale string, page, limit int) ([]models.Artist, int, error) {
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	artists, total, err := s.repo.FindAllPublished(ctx, locale, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.GetArtists: %w", err)
	}
	return artists, total, nil
}

func (s *Service) GetArtistBySlug(ctx context.Context, slug string, locale string) (*models.Artist, error) {
	if slug == "" {
		return nil, fmt.Errorf("service.GetArtistBySlug: slug cannot be empty")
	}
	locale, _ = i18ncontent.NormalizeLocale(locale, false)
	artist, err := s.repo.FindPublishedBySlug(ctx, locale, slug)
	if err != nil {
		return nil, fmt.Errorf("service.GetArtistBySlug: %w", err)
	}
	// hreflang alternates (PRD §4.4): locale→slug for every OTHER published
	// translation. Best-effort — a failure logs + leaves Alternates empty.
	alts, aerr := s.repo.FindPublishedAlternates(ctx, artist.ID, locale)
	if aerr != nil {
		log.Printf("artist.GetArtistBySlug.Alternates(%d): %v", artist.ID, aerr)
	} else if len(alts) > 0 {
		artist.Alternates = alts
	}
	// Load the ordered media gallery (detail view). The first item's media
	// PublicURL is the preferred avatar; AvatarURL is the fallback.
	if s.galleryLoader != nil {
		gallery, gerr := s.galleryLoader.ListArtistMedia(ctx, artist.ID)
		if gerr != nil {
			log.Printf("artist.GetArtistBySlug.Gallery(%d): %v", artist.ID, gerr)
		} else if len(gallery) > 0 {
			artist.Gallery = gallery
			if gallery[0].MediaAsset.PublicURL != "" {
				u := gallery[0].MediaAsset.PublicURL
				artist.AvatarURL = &u
			}
		}
	}
	return artist, nil
}

// --- Admin ---

func (s *Service) AdminListArtists(ctx context.Context, locale, status string, page, limit int) ([]models.Artist, int, error) {
	if locale != "" {
		var err error
		locale, err = i18ncontent.NormalizeLocale(locale, true)
		if err != nil {
			return nil, 0, models.ErrInvalidLocale
		}
	}
	return s.repo.FindAllAdmin(ctx, locale, status, page, limit)
}

func (s *Service) AdminGetArtist(ctx context.Context, slug string, locale string) (*models.Artist, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	return s.repo.FindAdminBySlug(ctx, locale, slug)
}

func (s *Service) AdminCreateArtist(ctx context.Context, data models.CreateArtistData) (*models.Artist, error) {
	locale, err := i18ncontent.NormalizeLocale(data.Locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	data.Locale = locale
	return s.repo.CreateWithTranslation(ctx, data)
}

func (s *Service) AdminUpdateArtist(ctx context.Context, artistID int64, locale string, data models.UpdateArtistData, actor i18ncontent.WorkflowActor) (*models.Artist, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	currentStatus, err := s.repo.GetTranslationStatus(ctx, artistID, locale)
	if err != nil {
		return nil, err
	}
	if !i18ncontent.CanEdit(currentStatus, actor) {
		return nil, models.ErrInvalidWorkflowTransition
	}
	return s.repo.UpdateTranslation(ctx, artistID, locale, data)
}

func (s *Service) AdminTransitionArtist(ctx context.Context, artistID int64, locale string, to models.ContentStatus, actor i18ncontent.WorkflowActor, reviewerID string) (*models.Artist, error) {
	locale, err := i18ncontent.NormalizeLocale(locale, true)
	if err != nil {
		return nil, models.ErrInvalidLocale
	}
	from, err := s.repo.GetTranslationStatus(ctx, artistID, locale)
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
	if err := s.repo.UpdateTranslationStatus(ctx, artistID, locale, to, reviewer); err != nil {
		return nil, err
	}
	if to == models.StatusPublished || from == models.StatusPublished {
		s.enqueueSitemapRebuild(ctx)
	}
	return s.repo.FindAdminByID(ctx, artistID, locale)
}

func (s *Service) AdminDeleteArtist(ctx context.Context, artistID int64) error {
	return s.repo.Delete(ctx, artistID)
}
