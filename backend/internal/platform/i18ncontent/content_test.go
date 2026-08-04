package i18ncontent

import (
	"errors"
	"testing"

	"jingdezhen-ceramics-backend/internal/models"
)

// TestTransitionMatrix exhaustively checks the content-workflow state machine
// (TDD §4.2). This is a priority test target: a wrong rule here would either
// let an editor publish (content integrity breach) or block the super admin
// from publishing (the site can't go live).
func TestTransitionMatrix(t *testing.T) {
	type tc struct {
		name    string
		from    models.ContentStatus
		to      models.ContentStatus
		actor   WorkflowActor
		wantErr bool
	}
	tests := []tc{
		// --- Editor ---
		{"editor submits draft for review", models.StatusDraft, models.StatusInReview, ActorEditor, false},
		{"editor edits rejected → draft", models.StatusRejected, models.StatusDraft, ActorEditor, false},
		{"editor CANNOT publish", models.StatusInReview, models.StatusPublished, ActorEditor, true},
		{"editor CANNOT reject", models.StatusInReview, models.StatusRejected, ActorEditor, true},
		{"editor CANNOT unpublish", models.StatusPublished, models.StatusDraft, ActorEditor, true},
		{"editor CANNOT publish a draft directly", models.StatusDraft, models.StatusPublished, ActorEditor, true},

		// --- Super admin (can act as editor + has exclusive powers) ---
		{"super admin approves", models.StatusInReview, models.StatusPublished, ActorSuperAdmin, false},
		{"super admin rejects", models.StatusInReview, models.StatusRejected, ActorSuperAdmin, false},
		{"super admin unpublishes", models.StatusPublished, models.StatusDraft, ActorSuperAdmin, false},
		{"super admin submits draft", models.StatusDraft, models.StatusInReview, ActorSuperAdmin, false},
		{"super admin reopens rejected", models.StatusRejected, models.StatusDraft, ActorSuperAdmin, false},

		// --- Disallowed transitions for everyone ---
		{"no one: published → in_review", models.StatusPublished, models.StatusInReview, ActorSuperAdmin, true},
		{"no one: published → rejected directly", models.StatusPublished, models.StatusRejected, ActorSuperAdmin, true},
		{"no one: in_review → draft (must be rejected first)", models.StatusInReview, models.StatusDraft, ActorEditor, true},
		{"no one: in_review → draft (super admin too)", models.StatusInReview, models.StatusDraft, ActorSuperAdmin, true},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got, err := Transition(c.from, c.to, c.actor)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Transition(%s→%s, %s): want error, got nil (new=%s)", c.from, c.to, c.actor, got)
				}
				if !errors.Is(err, models.ErrInvalidWorkflowTransition) {
					t.Fatalf("Transition(%s→%s, %s): want ErrInvalidWorkflowTransition, got %v", c.from, c.to, c.actor, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Transition(%s→%s, %s): want nil error, got %v", c.from, c.to, c.actor, err)
			}
			if got != c.to {
				t.Fatalf("Transition(%s→%s, %s): returned %s, want %s", c.from, c.to, c.actor, got, c.to)
			}
		})
	}
}

func TestCanEdit(t *testing.T) {
	tests := []struct {
		status models.ContentStatus
		actor  WorkflowActor
		want   bool
	}{
		{models.StatusDraft, ActorEditor, true},
		{models.StatusRejected, ActorEditor, true},
		{models.StatusInReview, ActorEditor, false},
		{models.StatusPublished, ActorEditor, false},
		// super admin can edit anything
		{models.StatusDraft, ActorSuperAdmin, true},
		{models.StatusInReview, ActorSuperAdmin, true},
		{models.StatusPublished, ActorSuperAdmin, true},
		{models.StatusRejected, ActorSuperAdmin, true},
	}
	for _, c := range tests {
		got := CanEdit(c.status, c.actor)
		if got != c.want {
			t.Errorf("CanEdit(%s, %s) = %v, want %v", c.status, c.actor, got, c.want)
		}
	}
}

func TestNormalizeLocale(t *testing.T) {
	// Public reads: unknown → default (no error).
	got, err := NormalizeLocale("xx-XX", false)
	if err != nil || got != models.DefaultLocale {
		t.Fatalf("NormalizeLocale(xx-XX, validate=false) = (%s, %v), want (%s, nil)", got, err, models.DefaultLocale)
	}
	// Empty → default.
	if got, _ := NormalizeLocale("", false); got != models.DefaultLocale {
		t.Fatalf("NormalizeLocale('') = %s, want %s", got, models.DefaultLocale)
	}
	// Supported → as-is.
	if got, _ := NormalizeLocale(models.LocaleZhCN, false); got != models.LocaleZhCN {
		t.Fatalf("NormalizeLocale(%s) = %s, want %s", models.LocaleZhCN, got, models.LocaleZhCN)
	}
	// CMS write: unknown → error.
	if _, err := NormalizeLocale("xx-XX", true); !errors.Is(err, models.ErrInvalidLocale) {
		t.Fatalf("NormalizeLocale(xx-XX, validate=true) err = %v, want ErrInvalidLocale", err)
	}
}
