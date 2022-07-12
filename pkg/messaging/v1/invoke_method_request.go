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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/dapr/dapr/pkg/config"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
)

const (
	// DefaultAPIVersion is the default Dapr API version.
	DefaultAPIVersion = internalv1pb.APIVersion_V1
)

// InvokeMethodRequest holds InternalInvokeRequest protobuf message
// and provides the helpers to manage it.
type InvokeMethodRequest struct {
	r    *internalv1pb.InternalInvokeRequest
	data io.ReadCloser
}

// NewInvokeMethodRequest creates InvokeMethodRequest object for method.
func NewInvokeMethodRequest(method string) *InvokeMethodRequest {
	return &InvokeMethodRequest{
		r: &internalv1pb.InternalInvokeRequest{
			Ver: DefaultAPIVersion,
			Message: &commonv1pb.InvokeRequest{
				Method: method,
			},
		},
	}
}

// FromInvokeRequestMessage creates InvokeMethodRequest object from InvokeRequest pb object.
func FromInvokeRequestMessage(pb *commonv1pb.InvokeRequest) *InvokeMethodRequest {
	req := &InvokeMethodRequest{
		r: &internalv1pb.InternalInvokeRequest{
			Ver:     DefaultAPIVersion,
			Message: pb,
		},
	}

	if pb != nil && pb.Data != nil && pb.Data.Value != nil {
		req.data = io.NopCloser(bytes.NewReader(pb.Data.Value))
		pb.Data.Reset()
	}

	return req
}

// InternalInvokeRequest creates InvokeMethodRequest object from InternalInvokeRequest pb object.
func InternalInvokeRequest(pb *internalv1pb.InternalInvokeRequest) (*InvokeMethodRequest, error) {
	req := &InvokeMethodRequest{r: pb}
	if pb.Message == nil {
		return nil, errors.New("Message field is nil")
	}

	if pb.Message.Data != nil && pb.Message.Data.Value != nil {
		req.data = io.NopCloser(bytes.NewReader(pb.Message.Data.Value))
		pb.Message.Data.Reset()
	}

	return req, nil
}

// Close the data stream.
func (imr *InvokeMethodRequest) Close() (err error) {
	if imr.data == nil {
		return nil
	}
	err = imr.data.Close()
	if err != nil {
		return err
	}
	imr.data = nil
	return nil
}

// WithActor sets actor type and id.
func (imr *InvokeMethodRequest) WithActor(actorType, actorID string) *InvokeMethodRequest {
	imr.r.Actor = &internalv1pb.Actor{ActorType: actorType, ActorId: actorID}
	return imr
}

// WithMetadata sets metadata.
func (imr *InvokeMethodRequest) WithMetadata(md map[string][]string) *InvokeMethodRequest {
	imr.r.Metadata = MetadataToInternalMetadata(md)
	return imr
}

// WithFastHTTPHeaders sets fasthttp request headers.
func (imr *InvokeMethodRequest) WithFastHTTPHeaders(header *fasthttp.RequestHeader) *InvokeMethodRequest {
	md := map[string][]string{}
	header.VisitAll(func(key []byte, value []byte) {
		md[string(key)] = []string{string(value)}
	})
	imr.r.Metadata = MetadataToInternalMetadata(md)
	return imr
}

// WithRawData sets message data and content_type.
func (imr *InvokeMethodRequest) WithRawData(data io.ReadCloser, contentType string) *InvokeMethodRequest {
	// TODO: Remove the entire block once feature is finalized
	if contentType == "" && !config.GetNoDefaultContentType() {
		contentType = JSONContentType
	}
	imr.r.Message.ContentType = contentType
	imr.data = data
	return imr
}

// WithRawDataBytes sets message data from a []byte and content_type.
func (imr *InvokeMethodRequest) WithRawDataBytes(data []byte, contentType string) *InvokeMethodRequest {
	return imr.WithRawData(io.NopCloser(bytes.NewReader(data)), contentType)
}

// WithRawDataString sets message data from a string and content_type.
func (imr *InvokeMethodRequest) WithRawDataString(data string, contentType string) *InvokeMethodRequest {
	return imr.WithRawData(io.NopCloser(strings.NewReader(data)), contentType)
}

// WithHTTPExtension sets new HTTP extension with verb and querystring.
func (imr *InvokeMethodRequest) WithHTTPExtension(verb string, querystring string) *InvokeMethodRequest {
	httpMethod, ok := commonv1pb.HTTPExtension_Verb_value[strings.ToUpper(verb)]
	if !ok {
		httpMethod = int32(commonv1pb.HTTPExtension_POST)
	}

	imr.r.Message.HttpExtension = &commonv1pb.HTTPExtension{
		Verb:        commonv1pb.HTTPExtension_Verb(httpMethod),
		Querystring: querystring,
	}

	return imr
}

// WithCustomHTTPMetadata applies a metadata map to a InvokeMethodRequest.
func (imr *InvokeMethodRequest) WithCustomHTTPMetadata(md map[string]string) *InvokeMethodRequest {
	for k, v := range md {
		if imr.r.Metadata == nil {
			imr.r.Metadata = make(map[string]*internalv1pb.ListStringValue)
		}

		// NOTE: We don't explicitly lowercase the keys here but this will be done
		//       later when attached to the HTTP request as headers.
		imr.r.Metadata[k] = &internalv1pb.ListStringValue{Values: []string{v}}
	}

	return imr
}

// EncodeHTTPQueryString generates querystring for http using http extension object.
func (imr *InvokeMethodRequest) EncodeHTTPQueryString() string {
	m := imr.r.Message
	if m == nil || m.GetHttpExtension() == nil {
		return ""
	}

	return m.GetHttpExtension().Querystring
}

// APIVersion gets API version of InvokeMethodRequest.
func (imr *InvokeMethodRequest) APIVersion() internalv1pb.APIVersion {
	return imr.r.GetVer()
}

// Metadata gets Metadata of InvokeMethodRequest.
func (imr *InvokeMethodRequest) Metadata() DaprInternalMetadata {
	return imr.r.GetMetadata()
}

// Proto returns InternalInvokeRequest Proto object.
func (imr *InvokeMethodRequest) Proto() *internalv1pb.InternalInvokeRequest {
	return imr.r
}

// ProtoWithData returns a copy of the internal InvokeMethodRequest Proto object with the entire data stream read into the Data property.
func (imr *InvokeMethodRequest) ProtoWithData() (*internalv1pb.InternalInvokeRequest, error) {
	var (
		data []byte
		err  error
	)
	if imr.data != nil {
		data, err = io.ReadAll(imr.data)
		if err != nil {
			return nil, err
		}
	}
	m := proto.Clone(imr.r).(*internalv1pb.InternalInvokeRequest)
	m.Message.Data = &anypb.Any{
		Value: data,
	}
	return m, nil
}

// Actor returns actor type and id.
func (imr *InvokeMethodRequest) Actor() *internalv1pb.Actor {
	return imr.r.GetActor()
}

// Message gets InvokeRequest Message object.
func (imr *InvokeMethodRequest) Message() *commonv1pb.InvokeRequest {
	return imr.r.Message
}

// HasMessageData returns true if the message object contains a slice of data
func (imr *InvokeMethodRequest) HasMessageData() bool {
	m := imr.r.Message
	return m != nil && m.Data != nil && len(m.Data.Value) > 0
}

// RawData returns content_type and stream body.
func (imr *InvokeMethodRequest) RawData() (string, io.Reader) {
	m := imr.r.Message
	if m == nil {
		return "", nil
	}

	contentType := m.GetContentType()

	// If the message has a data property, use that
	r := imr.data
	if imr.HasMessageData() {
		r = io.NopCloser(bytes.NewReader(m.Data.Value))
	}

	// TODO: Remove once feature is finalized
	if !config.GetNoDefaultContentType() {
		dataTypeURL := m.GetData().GetTypeUrl()
		// set content_type to application/json only if typeurl is unset and data is given
		if contentType == "" && (dataTypeURL == "" && r != nil) {
			contentType = JSONContentType
		}
	}

	if r == nil {
		r = io.NopCloser(bytes.NewReader(nil))
	}

	return contentType, r
}

// Adds a new header to the existing set.
func (imr *InvokeMethodRequest) AddHeaders(header *fasthttp.RequestHeader) {
	md := map[string][]string{}
	header.VisitAll(func(key []byte, value []byte) {
		md[string(key)] = []string{string(value)}
	})

	internalMd := MetadataToInternalMetadata(md)

	if imr.r.Metadata == nil {
		imr.r.Metadata = internalMd
	} else {
		for key, val := range internalMd {
			// We're only adding new values, not overwriting existing
			if _, ok := imr.r.Metadata[key]; !ok {
				imr.r.Metadata[key] = val
			}
		}
	}
}
