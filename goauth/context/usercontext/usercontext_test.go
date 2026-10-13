package usercontext_test

import (
	"context"
	"testing"

	"github.com/arashthr/goauth/context/usercontext"
	"github.com/arashthr/goauth/models"
)

func TestWithUser_And_User(t *testing.T) {
	u := &models.User{ID: 42, Email: "test@example.com"}
	ctx := usercontext.WithUser(context.Background(), u)

	got := usercontext.User(ctx)
	if got == nil {
		t.Fatal("User: got nil, want non-nil")
	}
	if got.ID != 42 {
		t.Errorf("User.ID: got %v, want 42", got.ID)
	}
}

func TestUser_NotSet_ReturnsNil(t *testing.T) {
	got := usercontext.User(context.Background())
	if got != nil {
		t.Errorf("User: got %v, want nil", got)
	}
}

func TestWithUser_Overwrite(t *testing.T) {
	u1 := &models.User{ID: 1}
	u2 := &models.User{ID: 2}
	ctx := usercontext.WithUser(context.Background(), u1)
	ctx = usercontext.WithUser(ctx, u2)

	got := usercontext.User(ctx)
	if got.ID != 2 {
		t.Errorf("User.ID after overwrite: got %v, want 2", got.ID)
	}
}
