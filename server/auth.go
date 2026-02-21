package main

import (
	"context"
	"log"
	"strings"

	"github.com/a2aproject/a2a-go/a2asrv"
)

// authInterceptor implements [a2asrv.CallInterceptor] to handle identity verification.
// It populates the CallContext.User object based on the incoming Authorization header.
type authInterceptor struct {
	a2asrv.PassthroughCallInterceptor
}

// Before is executed before every A2A protocol call.
func (i *authInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
        // Extract Authorization header from RequestMeta (Case-insensitive)
        authHeaders, ok := callCtx.ServiceParams().Get("Authorization")
        if ok && len(authHeaders) > 0 {
                // Basic demo logic: check for 'secret-token'
                token := strings.TrimPrefix(authHeaders[0], "Bearer ")
                if token == "secret-token" {
                        // Successfully authenticated
                        callCtx.User = a2asrv.NewAuthenticatedUser("Admin", nil)
                        log.Println("Request successfully authenticated as Admin")
                }
        }
        return ctx, nil, nil
}
