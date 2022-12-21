/*
Copyright 2021 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package http

import (
	"context"
	"fmt"
	"reflect"

	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
)

// FastHTTPHandler wraps a UniversalAPI method into a FastHTTP handler.
func FastHTTPHandler[T proto.Message, U proto.Message](method func(ctx context.Context, in T) (U, error)) func(reqCtx *fasthttp.RequestCtx) {
	var zero T
	rt := reflect.ValueOf(zero).Type().Elem()

	return func(reqCtx *fasthttp.RequestCtx) {
		// Read the response body and decode it as JSON using protojson
		body := reqCtx.PostBody()
		// Need to use some reflection magic to allocate a value to the pointer
		in := reflect.New(rt).Interface().(T)
		err := protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}.Unmarshal(body, in)
		if err != nil {
			msg := NewErrorResponse("ERR_MALFORMED_REQUEST", err.Error())
			respond(reqCtx, withError(fasthttp.StatusBadRequest, msg))
			log.Debug(msg)
			return
		}

		// Invoke the gRPC handler
		res, err := method(reqCtx, in)
		if err != nil {
			// Error is already logged
			msg := NewErrorResponse("ERROR", err.Error())
			sc := fasthttp.StatusInternalServerError
			s, ok := status.FromError(err)
			if ok && s != nil {
				sc = invokev1.HTTPStatusFromCode(s.Code())
			}
			respond(reqCtx, withError(sc, msg))
			return
		}

		// Encode the response as JSON using protojson
		respBytes, err := protojson.Marshal(res)
		if err != nil {
			err = fmt.Errorf("failed to encode response as JSON: %w", err)
			msg := NewErrorResponse("ERR_INTERNAL", err.Error())
			respond(reqCtx, withError(fasthttp.StatusInternalServerError, msg))
			log.Debug(msg)
			return
		}

		respond(reqCtx, withJSON(fasthttp.StatusOK, respBytes))
	}
}
