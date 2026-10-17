package api

import "context"

type contextKey struct{}

var userEmailKey = contextKey{}

func withEmail(ctx context.Context, userEmail string) context.Context {
	return context.WithValue(ctx, userEmailKey, userEmail)
}

func emailFromContext(ctx context.Context) string {
	userEmail, _ := ctx.Value(userEmailKey).(string)
	return userEmail
}
