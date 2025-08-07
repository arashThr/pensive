package usercontext

import (
	"context"

	"github.com/arashthr/go-course/internal/models"
)

type key string

const userKey key = "userKey"

func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func User(ctx context.Context) *models.User {
	val := ctx.Value(userKey)
	user, ok := val.(*models.User)
	if !ok {
		// Most likely user context was not set
		return nil
	}
	return user
}
