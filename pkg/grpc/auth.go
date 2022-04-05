package grpc

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	v1 "github.com/dapr/dapr/pkg/messaging/v1"
	auth "github.com/dapr/dapr/pkg/runtime/security"
)

func setAPIAuthenticationMiddlewareUnary(apiToken *auth.APIToken, authHeader string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			err := v1.ErrorFromHTTPResponseCode(http.StatusUnauthorized, "missing metadata in request")
			return nil, err
		}

		token := md.Get(authHeader)
		if len(token) == 0 {
			err := v1.ErrorFromHTTPResponseCode(http.StatusUnauthorized, "missing api token in request metadata")
			return nil, err
		}

		if !apiToken.CheckAPIToken([]byte(token[0])) {
			err := v1.ErrorFromHTTPResponseCode(http.StatusUnauthorized, "authentication error: api token mismatch")
			return nil, err
		}

		md.Set(authHeader, "")
		return handler(ctx, req)
	}
}
