package authz

import (
	"context"

	"github.com/pkg/errors"
)

type contextKey string

const (
	contextKeyUser contextKey = "authzUser"
)

func WithContextUser(parent context.Context, user User) context.Context {
	return context.WithValue(parent, contextKeyUser, user)
}

func ContextUser(ctx context.Context) (User, error) {
	raw := ctx.Value(contextKeyUser)
	if raw == nil {
		return nil, errors.New("no user in context")
	}

	user, ok := raw.(User)
	if !ok {
		return nil, errors.New("unexpected context value type for user")
	}

	return user, nil
}
