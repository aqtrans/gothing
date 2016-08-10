package httptreemux

import (
	"context"
	"net/http"
)

func ContextParams(ctx context.Context) map[string]string {
	if p, ok := ctx.Value(ParamsContextKey).(map[string]string); ok {
		return p
	}
	return nil
}

func ContextMethods(ctx context.Context) map[string]http.HandlerFunc {
	if p, ok := ctx.Value(methodsContextKey).(map[string]http.HandlerFunc); ok {
		return p
	}
	return nil
}

func ContextError(ctx context.Context) interface{} {
	return ctx.Value(errorContextKey)
}
