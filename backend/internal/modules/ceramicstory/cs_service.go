package ceramicstory

import (
	"context"
	"fmt"
	"jingdezhen-ceramics-backend/internal/models"
)

// ServiceInterface defines the methods for ceramic story business logic.
type ServiceInterface interface {
	GetAllCeramicStories(ctx context.Context) ([]models.CeramicStory, error)
	GetCeramicStoryDetail(ctx context.Context, idOrSlug string) (*models.CeramicStory, error)
	// Admin methods
	// CreateCeramicStory(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error)
	// UpdateCeramicStory(ctx context.Context, id int64, data models.UpdateCeramicStoryData) (*models.CeramicStory, error)
	// DeleteCeramicStory(ctx context.Context, id int64) error
}

// Service provides business logic for ceramic stories.
type Service struct {
	repo RepositoryInterface
}

// NewService creates a new ceramic story service.
func NewService(repo RepositoryInterface) ServiceInterface {
	return &Service{repo: repo}
}

// GetAllCeramicStories retrieves all ceramic stories.
func (s *Service) GetAllCeramicStories(ctx context.Context) ([]models.CeramicStory, error) {
	stories, err := s.repo.FindAll(ctx)
	if err != nil {
		// In a more complex scenario, you might map repository errors to service-level errors
		return nil, fmt.Errorf("service.GetAllCeramicStories: %w", err)
	}
	// Add any business logic here if needed (e.g., filtering, transformations)
	// For this simple case, we directly return the repository result.
	return stories, nil
}

// GetCeramicStoryDetail retrieves details for a specific ceramic story by ID or slug.
func (s *Service) GetCeramicStoryDetail(ctx context.Context, idOrSlug string) (*models.CeramicStory, error) {
	if idOrSlug == "" {
		return nil, fmt.Errorf("service.GetCeramicStoryDetail: idOrSlug cannot be empty") // Basic validation
	}
	story, err := s.repo.FindByIDOrSlug(ctx, idOrSlug)
	if err != nil {
		return nil, fmt.Errorf("service.GetCeramicStoryDetail: %w", err)
	}
	// Add any business logic here (e.g., increment view count, fetch related data)
	return story, nil
}

// --- Admin Service Methods (Example Implementations - uncomment and complete if needed for admin panel) ---
/*
func (s *Service) CreateCeramicStory(ctx context.Context, data models.CreateCeramicStoryData) (*models.CeramicStory, error) {
	// Add validation logic here if not fully covered by handler/validator (e.g., check for slug uniqueness)
	// existingBySlug, _ := s.repo.FindByIDOrSlug(ctx, data.Slug)
	// if existingBySlug != nil {
	// 	 return nil, models.ErrConflict // Or a more specific "slug taken" error
	// }
	return s.repo.Create(ctx, data)
}

func (s *Service) UpdateCeramicStory(ctx context.Context, id int64, data models.UpdateCeramicStoryData) (*models.CeramicStory, error) {
	// Add validation logic (e.g., if slug is being changed, check for uniqueness)
	// if data.Slug != nil {
	// 	 existingBySlug, _ := s.repo.FindByIDOrSlug(ctx, *data.Slug)
	// 	 if existingBySlug != nil && existingBySlug.ID != id {
	// 	 	return nil, models.ErrConflict // Slug taken by another story
	// 	 }
	// }
	return s.repo.Update(ctx, id, data)
}

func (s *Service) DeleteCeramicStory(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
*/
