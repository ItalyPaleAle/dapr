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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"net/http"
	nethttp "net/http"

	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapr/dapr/pkg/channel"
	"github.com/dapr/dapr/pkg/config"
	diag "github.com/dapr/dapr/pkg/diagnostics"
	diag_utils "github.com/dapr/dapr/pkg/diagnostics/utils"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
	auth "github.com/dapr/dapr/pkg/runtime/security"
)

const (
	// HTTPStatusCode is an dapr http channel status code.
	HTTPStatusCode = "http.status_code"
	httpScheme     = "http"
	httpsScheme    = "https"

	appConfigEndpoint = "dapr/config"
)

// Channel is an HTTP implementation of an AppChannel.
type Channel struct {
	client              *http.Client
	baseAddress         string
	ch                  chan struct{}
	tracingSpec         config.TracingSpec
	appHeaderToken      string
	maxResponseBodySize int
}

// CreateLocalChannel creates an HTTP AppChannel
// nolint:gosec
func CreateLocalChannel(port, maxConcurrency int, spec config.TracingSpec, sslEnabled bool, maxRequestBodySize int, readBufferSize int) (channel.AppChannel, error) {
	scheme := httpScheme
	if sslEnabled {
		scheme = httpsScheme
	}

	c := &Channel{
		// We cannot use fasthttp here because of lack of streaming support
		client: &http.Client{
			Transport: &http.Transport{
				ReadBufferSize:         readBufferSize * 1024,
				MaxResponseHeaderBytes: int64(readBufferSize) * 1024,
			},
		},
		baseAddress:         fmt.Sprintf("%s://%s:%d", scheme, channel.DefaultChannelAddress, port),
		tracingSpec:         spec,
		appHeaderToken:      auth.GetAppToken(),
		maxResponseBodySize: maxRequestBodySize,
	}

	if sslEnabled {
		(c.client.Transport.(*http.Transport)).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	if maxConcurrency > 0 {
		c.ch = make(chan struct{}, maxConcurrency)
	}

	return c, nil
}

// GetBaseAddress returns the application base address.
func (h *Channel) GetBaseAddress() string {
	return h.baseAddress
}

// GetAppConfig gets application config from user application
// GET http://localhost:<app_port>/dapr/config
func (h *Channel) GetAppConfig() (*config.ApplicationConfig, error) {
	req := invokev1.NewInvokeMethodRequest(appConfigEndpoint)
	req.WithHTTPExtension(nethttp.MethodGet, "")
	req.WithRawData(nil, invokev1.JSONContentType)

	// TODO Propagate context
	ctx := context.Background()
	resp, err := h.InvokeMethod(ctx, req)
	if err != nil {
		return nil, err
	}

	var config config.ApplicationConfig

	if resp.Status().Code != nethttp.StatusOK {
		return &config, nil
	}

	// Get versioning info, currently only v1 is supported.
	headers := resp.Headers()
	var version string
	if val, ok := headers["dapr-app-config-version"]; ok {
		if len(val.Values) == 1 {
			version = val.Values[0]
		}
	}

	switch version {
	case "v1":
		fallthrough
	default:
		_, r := resp.RawData()
		if err = json.NewDecoder(r).Decode(&config); err != nil {
			return nil, err
		}
	}

	return &config, nil
}

// InvokeMethod invokes user code via HTTP.
func (h *Channel) InvokeMethod(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	// Check if HTTP Extension is given. Otherwise, it will return error.
	httpExt := req.Message().GetHttpExtension()
	if httpExt == nil {
		return nil, status.Error(codes.InvalidArgument, "missing HTTP extension field")
	}
	if httpExt.GetVerb() == commonv1pb.HTTPExtension_NONE {
		return nil, status.Error(codes.InvalidArgument, "invalid HTTP verb")
	}

	var rsp *invokev1.InvokeMethodResponse
	var err error
	switch req.APIVersion() {
	case internalv1pb.APIVersion_V1:
		rsp, err = h.invokeMethodV1(ctx, req)

	default:
		// Reject unsupported version
		err = status.Error(codes.Unimplemented, fmt.Sprintf("Unsupported spec version: %d", req.APIVersion()))
	}

	return rsp, err
}

func (h *Channel) invokeMethodV1(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	channelReq, err := h.constructRequest(ctx, req)

	if h.ch != nil {
		h.ch <- struct{}{}
	}
	defer func() {
		if h.ch != nil {
			<-h.ch
		}
	}()

	// Emit metric when request is sent
	diag.DefaultHTTPMonitoring.ClientRequestStarted(ctx, channelReq.Method, req.Message().Method, int64(len(req.Message().Data.GetValue())))
	startRequest := time.Now()

	// Send request to user application
	resp, err := h.client.Do(channelReq)

	elapsedMs := float64(time.Since(startRequest) / time.Millisecond)

	contentLength, _ := strconv.Atoi(resp.Header.Get("content-length"))
	if err != nil {
		diag.DefaultHTTPMonitoring.ClientRequestCompleted(ctx, channelReq.Method, req.Message().GetMethod(), strconv.Itoa(nethttp.StatusInternalServerError), int64(contentLength), elapsedMs)
		return nil, err
	}

	rsp := h.parseChannelResponse(req, resp)
	diag.DefaultHTTPMonitoring.ClientRequestCompleted(ctx, channelReq.Method, req.Message().GetMethod(), strconv.Itoa(int(rsp.Status().Code)), int64(contentLength), elapsedMs)

	return rsp, nil
}

func (h *Channel) constructRequest(ctx context.Context, req *invokev1.InvokeMethodRequest) (*http.Request, error) {
	// Construct app channel URI: VERB http://localhost:3000/method?query1=value1
	var uri string
	method := req.Message().HttpExtension.Verb.String()
	if strings.HasPrefix(method, "/") {
		uri = fmt.Sprintf("%s%s", h.baseAddress, method)
	} else {
		uri = fmt.Sprintf("%s/%s", h.baseAddress, method)
	}
	qs := req.EncodeHTTPQueryString()
	if qs != "" {
		uri += "?" + qs
	}

	contentType, body := req.RawData()
	channelReq, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, err
	}

	// Recover headers
	invokev1.InternalMetadataToHTTPHeader(ctx, req.Metadata(), channelReq.Header.Set)
	channelReq.Header.Set("content-type", contentType)

	// HTTP client needs to inject traceparent header for proper tracing stack.
	span := diag_utils.SpanFromContext(ctx)
	httpFormat := &tracecontext.HTTPFormat{}
	tp, ts := httpFormat.SpanContextToHeaders(span.SpanContext())
	channelReq.Header.Set("traceparent", tp)
	if ts != "" {
		channelReq.Header.Set("tracestate", ts)
	}

	if h.appHeaderToken != "" {
		channelReq.Header.Set(auth.APITokenHeader, h.appHeaderToken)
	}

	return channelReq, nil
}

func (h *Channel) parseChannelResponse(req *invokev1.InvokeMethodRequest, resp *http.Response) *invokev1.InvokeMethodResponse {
	var contentType string

	contentType = resp.Header.Get("content-type")
	// TODO: Remove entire block when feature is finalized
	if contentType == "" && !config.GetNoDefaultContentType() {
		contentType = "text/plain; charset=utf-8"
	}

	var body io.ReadCloser = resp.Body
	// We are not limiting the response body because we use streams
	/*// Limit response body if needed
	if h.maxResponseBodySize > 0 {
		body = LimitReadCloser(body, int64(h.maxResponseBodySize)*1024*1024)
	}*/

	// Convert status code
	rsp := invokev1.
		NewInvokeMethodResponse(int32(resp.StatusCode), "", nil).
		WithHTTPHeaders(resp.Header).
		WithRawData(body, contentType)

	return rsp
}

/*!
Adapted from the Go 1.18.3 source code
Copyright 2009 The Go Authors. All rights reserved.
License: BSD (https://github.com/golang/go/blob/go1.18.3/LICENSE)
*/

// LimitReadCloser returns a ReadCloser that reads from r but stops with EOF after n bytes.
func LimitReadCloser(r io.ReadCloser, n int64) io.ReadCloser { return &limitReadCloser{r, n} }

type limitReadCloser struct {
	R io.ReadCloser
	N int64
}

func (l *limitReadCloser) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

func (l *limitReadCloser) Close() error {
	return l.R.Close()
}
