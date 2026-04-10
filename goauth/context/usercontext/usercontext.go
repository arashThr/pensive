package usercontext

import (
	"context"

	"github.com/arashthr/goauth/models"
)

type key string

const userKey key = "userKey"

// WithUser stores the user in the context.
func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// User retrieves the user from the context. Returns nil if not set.
func User(ctx context.Context) *models.User {
	val := ctx.Value(userKey)
	user, ok := val.(*models.User)
	if !ok {
		return nil
	}
	return user
}
