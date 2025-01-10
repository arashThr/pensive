package context

import (
	"context"

	"github.com/arashthr/go-course/models"
)

type key int

const (
	userKey key = iota
)

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
