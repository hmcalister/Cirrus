package api

import "context"

type contextKey struct{}

var (
	userEmailKey = contextKey{}
	requestIDKey = contextKey{}
)

func withEmail(ctx context.Context, userEmail string) context.Context {
	return context.WithValue(ctx, userEmailKey, userEmail)
}

func emailFromContext(ctx context.Context) string {
	userEmail, _ := ctx.Value(userEmailKey).(string)
	return userEmail
}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}
