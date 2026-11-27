// Package authctx carries request-scoped authentication facts through a
// context.Context, so layers that only receive a ctx (services, the database
// audit writer) can tell how the request was authenticated.
package authctx

import "context"

type apiKeyIDKey struct{}

// WithAPIKeyID returns a context recording that the request was authenticated
// with the API key of this id.
func WithAPIKeyID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, apiKeyIDKey{}, id)
}

// APIKeyID returns the id of the API key that authenticated the request, if
// any.
func APIKeyID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(apiKeyIDKey{}).(int64)
	return id, ok
}
