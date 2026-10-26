package api

import (
	"context"

	"github.com/hmcalister/cirrus/internal/database"
)

type contextKey struct{}

var (
	userKey      = contextKey{}
	requestIDKey = contextKey{}
)

func withUser(ctx context.Context, user database.UserAccount) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (database.UserAccount, bool) {
	user, ok := ctx.Value(userKey).(database.UserAccount)
	return user, ok
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}
