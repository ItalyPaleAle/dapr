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

package v1

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/valyala/fasthttp"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/dapr/dapr/pkg/config"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
)

// InvokeMethodResponse holds InternalInvokeResponse protobuf message
// and provides the helpers to manage it.
type InvokeMethodResponse struct {
	replayableRequest

	r *internalv1pb.InternalInvokeResponse
}

// NewInvokeMethodResponse returns new InvokeMethodResponse object with status.
func NewInvokeMethodResponse(statusCode int32, statusMessage string, statusDetails []*anypb.Any) *InvokeMethodResponse {
	return &InvokeMethodResponse{
		r: &internalv1pb.InternalInvokeResponse{
			Status:  &internalv1pb.Status{Code: statusCode, Message: statusMessage, Details: statusDetails},
			Message: &commonv1pb.InvokeResponse{},
		},
	}
}

// InternalInvokeResponse returns InvokeMethodResponse for InternalInvokeResponse pb to use the helpers.
func InternalInvokeResponse(pb *internalv1pb.InternalInvokeResponse) (*InvokeMethodResponse, error) {
	rsp := &InvokeMethodResponse{r: pb}
	if pb.Message == nil {
		pb.Message = &commonv1pb.InvokeResponse{Data: nil}
	} else if pb.Message.Data != nil && pb.Message.Data.Value != nil {
		rsp.data = io.NopCloser(bytes.NewReader(pb.Message.Data.Value))
		pb.Message.Data.Reset()
	}

	return rsp, nil
}

// WithMessage sets InvokeResponse pb object to Message field.
func (imr *InvokeMethodResponse) WithMessage(pb *commonv1pb.InvokeResponse) *InvokeMethodResponse {
	imr.r.Message = pb
	return imr
}

// WithRawData sets message data and content_type.
func (imr *InvokeMethodResponse) WithRawData(data io.ReadCloser, contentType string) *InvokeMethodResponse {
	if imr.replay != nil {
		// We are panicking here because we can't return errors
		// This is just to catch issues during development however, and will never happen at runtime
		panic("WithRawData cannot be invoked after replaying has been enabled")
	}

	// TODO: Remove the entire block once feature is finalized
	if contentType == "" && !config.GetNoDefaultContentType() {
		contentType = JSONContentType
	}

	imr.r.Message.ContentType = contentType
	imr.data = data

	return imr
}

// WithRawDataBytes sets message data from a []byte and content_type.
func (imr *InvokeMethodResponse) WithRawDataBytes(data []byte, contentType string) *InvokeMethodResponse {
	return imr.WithRawData(io.NopCloser(bytes.NewReader(data)), contentType)
}

// WithRawDataString sets message data from a string and content_type.
func (imr *InvokeMethodResponse) WithRawDataString(data string, contentType string) *InvokeMethodResponse {
	return imr.WithRawData(io.NopCloser(strings.NewReader(data)), contentType)
}

// WithHeaders sets gRPC response header metadata.
func (imr *InvokeMethodResponse) WithHeaders(headers metadata.MD) *InvokeMethodResponse {
	imr.r.Headers = MetadataToInternalMetadata(headers)
	return imr
}

// WithFastHTTPHeaders populates HTTP response header to gRPC header metadata.
func (imr *InvokeMethodResponse) WithHTTPHeaders(headers map[string][]string) *InvokeMethodResponse {
	imr.r.Headers = MetadataToInternalMetadata(headers)
	return imr
}

// WithFastHTTPHeaders populates fasthttp response header to gRPC header metadata.
func (imr *InvokeMethodResponse) WithFastHTTPHeaders(header *fasthttp.ResponseHeader) *InvokeMethodResponse {
	md := DaprInternalMetadata{}
	header.VisitAll(func(key []byte, value []byte) {
		md[string(key)] = &internalv1pb.ListStringValue{
			Values: []string{string(value)},
		}
	})
	if len(md) > 0 {
		imr.r.Headers = md
	}
	return imr
}

// WithTrailers sets Trailer in internal InvokeMethodResponse.
func (imr *InvokeMethodResponse) WithTrailers(trailer metadata.MD) *InvokeMethodResponse {
	imr.r.Trailers = MetadataToInternalMetadata(trailer)
	return imr
}

// WithReplay enables replaying for the data stream.
func (imr *InvokeMethodResponse) WithReplay(enabled bool) *InvokeMethodResponse {
	imr.replayableRequest.SetReplay(enabled)
	return imr
}

// Status gets Response status.
func (imr *InvokeMethodResponse) Status() *internalv1pb.Status {
	return imr.r.GetStatus()
}

// IsHTTPResponse returns true if response status code is http response status.
func (imr *InvokeMethodResponse) IsHTTPResponse() bool {
	// gRPC status code <= 15 - https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	// HTTP status code >= 100 - https://tools.ietf.org/html/rfc2616#section-10
	return imr.r.GetStatus().Code >= 100
}

// Proto returns the internal InvokeMethodResponse Proto object.
func (imr *InvokeMethodResponse) Proto() *internalv1pb.InternalInvokeResponse {
	return imr.r
}

// ProtoWithData returns a copy of the internal InvokeMethodResponse Proto object with the entire data stream read into the Data property.
func (imr *InvokeMethodResponse) ProtoWithData() (*internalv1pb.InternalInvokeResponse, error) {
	if imr.r == nil || imr.r.Message == nil {
		return nil, errors.New("message is nil")
	}

	var (
		data []byte
		err  error
	)
	if imr.data != nil {
		data, err = io.ReadAll(imr.RawData())
		if err != nil {
			return nil, err
		}
	}
	m := proto.Clone(imr.r).(*internalv1pb.InternalInvokeResponse)
	m.Message.Data = &anypb.Any{
		Value: data,
	}
	return m, nil
}

// Headers gets Headers metadata.
func (imr *InvokeMethodResponse) Headers() DaprInternalMetadata {
	return imr.r.Headers
}

// Trailers gets Trailers metadata.
func (imr *InvokeMethodResponse) Trailers() DaprInternalMetadata {
	return imr.r.Trailers
}

// Message returns message field in InvokeMethodResponse.
func (imr *InvokeMethodResponse) Message() *commonv1pb.InvokeResponse {
	return imr.r.Message
}

// HasMessageData returns true if the message object contains a slice of data.
func (imr *InvokeMethodResponse) HasMessageData() bool {
	m := imr.r.Message
	return m != nil && m.Data != nil && len(m.Data.Value) > 0
}

// ContenType returns the content type of the message.
func (imr *InvokeMethodResponse) ContentType() string {
	m := imr.r.Message
	if m == nil {
		return ""
	}

	contentType := m.GetContentType()
	dataTypeURL := m.GetData().GetTypeUrl()

	// TODO: Remove once feature is finalized
	if !config.GetNoDefaultContentType() {
		// set content_type to application/json only if typeurl is unset and data is given
		hasData := imr.data != nil && !imr.HasMessageData()
		if contentType == "" && (dataTypeURL == "" && hasData) {
			contentType = JSONContentType
		}
	}

	if dataTypeURL != "" {
		contentType = ProtobufContentType
	}

	return contentType
}

// RawData returns the stream body.
func (imr *InvokeMethodResponse) RawData() (r io.Reader) {
	m := imr.r.Message
	if m == nil {
		return nil
	}

	// If the message has a data property, use that
	if imr.HasMessageData() {
		return bytes.NewReader(m.Data.Value)
	}

	return imr.replayableRequest.RawData()
}
