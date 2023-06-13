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

//nolint:nosnakecase
package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/types/known/anypb"

	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
)

func TestInvokeRequest(t *testing.T) {
	req := NewInvokeMethodRequest("test_method")
	defer req.Close()

	assert.Equal(t, internalv1pb.APIVersion_V1, req.r.GetVer())
	assert.Equal(t, "test_method", req.r.Message.GetMethod())
}

func TestFromInvokeRequestMessage(t *testing.T) {
	t.Run("no data", func(t *testing.T) {
		pb := &commonv1pb.InvokeRequest{Method: "frominvokerequestmessage"}
		req := FromInvokeRequestMessage(pb)
		defer req.Close()

		assert.Equal(t, internalv1pb.APIVersion_V1, req.r.GetVer())
		assert.Equal(t, "frominvokerequestmessage", req.r.Message.GetMethod())

		bData, err := io.ReadAll(req.RawData())
		assert.NoError(t, err)
		assert.Len(t, bData, 0)
	})

	t.Run("with data", func(t *testing.T) {
		pb := &commonv1pb.InvokeRequest{
			Method: "frominvokerequestmessage",
			Data:   &anypb.Any{Value: []byte("test")},
		}
		req := FromInvokeRequestMessage(pb)
		defer req.Close()

		assert.Equal(t, internalv1pb.APIVersion_V1, req.r.GetVer())
		assert.Equal(t, "frominvokerequestmessage", req.r.Message.GetMethod())

		bData, err := io.ReadAll(req.RawData())
		assert.NoError(t, err)
		assert.Equal(t, "test", string(bData))
	})
}

func TestInternalInvokeRequest(t *testing.T) {
	t.Run("valid internal invoke request with no data", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
			Data:        nil,
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		assert.NotNil(t, ir.r.Message)
		assert.Equal(t, "invoketest", ir.r.Message.Method)
		assert.Nil(t, ir.r.Message.Data)

		bData, err := io.ReadAll(ir.RawData())
		assert.NoError(t, err)
		assert.Len(t, bData, 0)
	})

	t.Run("valid internal invoke request with data", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
			Data:        &anypb.Any{Value: []byte("test")},
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		assert.NotNil(t, ir.r.Message)
		assert.Equal(t, "invoketest", ir.r.Message.Method)
		require.NotNil(t, ir.r.Message.Data)
		require.NotNil(t, ir.r.Message.Data.Value)
		assert.Equal(t, []byte("test"), ir.r.Message.Data.Value)

		bData, err := io.ReadAll(ir.RawData())
		assert.NoError(t, err)
		assert.Equal(t, "test", string(bData))
	})

	t.Run("nil message field", func(t *testing.T) {
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: nil,
		}

		_, err := InternalInvokeRequest(&pb)
		assert.Error(t, err)
	})
}

func TestMetadata(t *testing.T) {
	t.Run("gRPC headers", func(t *testing.T) {
		md := map[string][]string{
			"test1": {"val1", "val2"},
			"test2": {"val3", "val4"},
		}
		req := NewInvokeMethodRequest("test_method").
			WithMetadata(md)
		defer req.Close()
		mdata := req.Metadata()

		assert.Equal(t, "val1", mdata["test1"].GetValues()[0])
		assert.Equal(t, "val2", mdata["test1"].GetValues()[1])
		assert.Equal(t, "val3", mdata["test2"].GetValues()[0])
		assert.Equal(t, "val4", mdata["test2"].GetValues()[1])
	})

	t.Run("HTTP headers", func(t *testing.T) {
		req := fasthttp.AcquireRequest()
		req.Header.Set("Header1", "Value1")
		req.Header.Set("Header2", "Value2")
		req.Header.Set("Header3", "Value3")

		re := NewInvokeMethodRequest("test_method").
			WithFastHTTPHeaders(&req.Header)
		defer re.Close()
		mheader := re.Metadata()

		assert.Equal(t, "Value1", mheader["Header1"].GetValues()[0])
		assert.Equal(t, "Value2", mheader["Header2"].GetValues()[0])
		assert.Equal(t, "Value3", mheader["Header3"].GetValues()[0])
	})
}

func TestData(t *testing.T) {
	t.Run("contenttype is set", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("test").
			WithContentType("application/json")
		defer req.Close()
		contentType := req.ContentType()
		bData, err := io.ReadAll(req.RawData())
		assert.NoError(t, err)
		assert.Equal(t, "application/json", contentType)
		assert.Equal(t, "test", string(bData))
	})

	t.Run("contenttype is unset,", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("test")
		defer req.Close()

		contentType := req.ContentType()
		bData, err := io.ReadAll(req.RawData())
		assert.NoError(t, err)
		assert.Equal(t, "", req.r.Message.ContentType)
		assert.Equal(t, "", contentType)
		assert.Equal(t, "test", string(bData))
	})

	t.Run("typeurl is set but content_type is unset", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		defer req.Close()
		req.r.Message.Data = &anypb.Any{TypeUrl: "fake", Value: []byte("fake")}
		contentType := req.ContentType()
		bData, err := io.ReadAll(req.RawData())
		assert.NoError(t, err)
		assert.Equal(t, "", contentType)
		assert.Equal(t, "fake", string(bData))
	})
}

func TestRawData(t *testing.T) {
	t.Run("message is nil", func(t *testing.T) {
		req := &InvokeMethodRequest{
			r: &internalv1pb.InternalInvokeRequest{},
		}
		r := req.RawData()
		assert.Nil(t, r)
	})

	t.Run("return data from stream", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("nel blu dipinto di blu")
		defer req.Close()

		r := req.RawData()
		bData, err := io.ReadAll(r)

		assert.NoError(t, err)
		assert.Equal(t, "nel blu dipinto di blu", string(bData))

		_ = assert.Nil(t, req.Message().Data) ||
			assert.Len(t, req.Message().Data.Value, 0)
	})

	t.Run("data inside message has priority", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("nel blu dipinto di blu")
		defer req.Close()

		// Override
		const msg = "felice di stare lassu'"
		req.Message().Data = &anypb.Any{Value: []byte(msg)}

		r := req.RawData()
		bData, err := io.ReadAll(r)

		assert.NoError(t, err)
		assert.Equal(t, msg, string(bData))

		_ = assert.NotNil(t, req.Message().Data) &&
			assert.Equal(t, msg, string(req.Message().Data.Value))
	})
}

func TestRawDataFull(t *testing.T) {
	t.Run("message is nil", func(t *testing.T) {
		req := &InvokeMethodRequest{
			r: &internalv1pb.InternalInvokeRequest{},
		}
		data, err := req.RawDataFull()
		assert.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("return data from stream", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("nel blu dipinto di blu")
		defer req.Close()

		data, err := req.RawDataFull()
		assert.NoError(t, err)
		assert.Equal(t, "nel blu dipinto di blu", string(data))

		_ = assert.Nil(t, req.Message().Data) ||
			assert.Len(t, req.Message().Data.Value, 0)
	})

	t.Run("data inside message has priority", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method").
			WithRawDataString("nel blu dipinto di blu")
		defer req.Close()

		// Override
		const msg = "felice di stare lassu'"
		req.Message().Data = &anypb.Any{Value: []byte(msg)}

		data, err := req.RawDataFull()
		assert.NoError(t, err)
		assert.Equal(t, msg, string(data))

		_ = assert.NotNil(t, req.Message().Data) &&
			assert.Equal(t, msg, string(req.Message().Data.Value))
	})
}

func TestHTTPExtension(t *testing.T) {
	req := NewInvokeMethodRequest("test_method").
		WithHTTPExtension("POST", "query1=value1&query2=value2")
	defer req.Close()
	assert.Equal(t, commonv1pb.HTTPExtension_POST, req.Message().HttpExtension.Verb)
	assert.Equal(t, "query1=value1&query2=value2", req.EncodeHTTPQueryString())
}

func TestActor(t *testing.T) {
	req := NewInvokeMethodRequest("test_method").
		WithActor("testActor", "1")
	defer req.Close()
	assert.Equal(t, "testActor", req.Actor().ActorType)
	assert.Equal(t, "1", req.Actor().GetActorId())
}

func TestRequestProto(t *testing.T) {
	t.Run("byte slice", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
			Data:        &anypb.Any{Value: []byte("test")},
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		req2 := ir.Proto()
		msg := req2.GetMessage()

		assert.Equal(t, "application/json", msg.ContentType)
		require.NotNil(t, msg.Data)
		require.NotNil(t, msg.Data.Value)
		assert.Equal(t, []byte("test"), msg.Data.Value)

		bData, err := io.ReadAll(ir.RawData())
		assert.NoError(t, err)
		assert.Equal(t, []byte("test"), bData)
	})

	t.Run("stream", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		ir.data = io.NopCloser(strings.NewReader("test"))
		req2 := ir.Proto()

		assert.Equal(t, "application/json", req2.GetMessage().ContentType)
		assert.Nil(t, req2.GetMessage().Data)

		bData, err := io.ReadAll(ir.RawData())
		assert.NoError(t, err)
		assert.Equal(t, []byte("test"), bData)
	})
}

func TestRequestProtoWithData(t *testing.T) {
	t.Run("byte slice", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
			Data:        &anypb.Any{Value: []byte("test")},
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		req2, err := ir.ProtoWithData()
		assert.NoError(t, err)

		assert.Equal(t, "application/json", req2.GetMessage().ContentType)
		assert.Equal(t, []byte("test"), req2.GetMessage().Data.Value)
	})

	t.Run("stream", func(t *testing.T) {
		m := &commonv1pb.InvokeRequest{
			Method:      "invoketest",
			ContentType: "application/json",
		}
		pb := internalv1pb.InternalInvokeRequest{
			Ver:     internalv1pb.APIVersion_V1,
			Message: m,
		}

		ir, err := InternalInvokeRequest(&pb)
		assert.NoError(t, err)
		defer ir.Close()
		ir.data = io.NopCloser(strings.NewReader("test"))
		req2, err := ir.ProtoWithData()
		assert.NoError(t, err)

		assert.Equal(t, "application/json", req2.GetMessage().ContentType)
		assert.Equal(t, []byte("test"), req2.GetMessage().Data.Value)
	})
}

func TestAddHeaders(t *testing.T) {
	t.Run("single value", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		defer req.Close()
		header := http.Header{}
		header.Add("Dapr-Reentrant-Id", "test")
		req.AddMetadata(header)

		require.NotNil(t, req.r.Metadata)
		require.NotNil(t, req.r.Metadata["Dapr-Reentrant-Id"])
		require.NotEmpty(t, req.r.Metadata["Dapr-Reentrant-Id"].Values)
		assert.Equal(t, "test", req.r.Metadata["Dapr-Reentrant-Id"].Values[0])
	})

	t.Run("multiple values", func(t *testing.T) {
		req := NewInvokeMethodRequest("test_method")
		defer req.Close()
		header := http.Header{}
		header.Add("Dapr-Reentrant-Id", "test")
		header.Add("Dapr-Reentrant-Id", "test2")
		req.AddMetadata(header)

		require.NotNil(t, req.r.Metadata)
		require.NotNil(t, req.r.Metadata["Dapr-Reentrant-Id"])
		require.NotEmpty(t, req.r.Metadata["Dapr-Reentrant-Id"].Values)
		assert.Equal(t, []string{"test", "test2"}, req.r.Metadata["Dapr-Reentrant-Id"].Values)
	})

	t.Run("does not overwrite", func(t *testing.T) {
		header := http.Header{}
		header.Add("Dapr-Reentrant-Id", "test")
		req := NewInvokeMethodRequest("test_method").WithHTTPHeaders(header)
		defer req.Close()

		header.Set("Dapr-Reentrant-Id", "test2")
		req.AddMetadata(header)

		require.NotNil(t, req.r.Metadata["Dapr-Reentrant-Id"])
		require.NotEmpty(t, req.r.Metadata["Dapr-Reentrant-Id"].Values)
		assert.Equal(t, "test", req.r.Metadata["Dapr-Reentrant-Id"].Values[0])
	})
}

func TestWithCustomHTTPMetadata(t *testing.T) {
	customMetadataKey := func(i int) string {
		return fmt.Sprintf("customMetadataKey%d", i)
	}
	customMetadataValue := func(i int) string {
		return fmt.Sprintf("customMetadataValue%d", i)
	}

	numMetadata := 10
	md := make(map[string]string, numMetadata)
	for i := 0; i < numMetadata; i++ {
		md[customMetadataKey(i)] = customMetadataValue(i)
	}

	req := NewInvokeMethodRequest("test_method").
		WithCustomHTTPMetadata(md)
	defer req.Close()

	imrMd := req.Metadata()
	for i := 0; i < numMetadata; i++ {
		val, ok := imrMd[customMetadataKey(i)]
		assert.True(t, ok)
		// We assume only 1 value per key as the input map can only support string -> string mapping.
		assert.Equal(t, customMetadataValue(i), val.Values[0])
	}
}

func TestWithDataObject(t *testing.T) {
	type testData struct {
		Str string `json:"str"`
		Int int    `json:"int"`
	}
	const expectJSON = `{"str":"mystring","int":42}`

	req := NewInvokeMethodRequest("test_method").
		WithDataObject(&testData{
			Str: "mystring",
			Int: 42,
		})

	got := req.GetDataObject()
	require.NotNil(t, got)

	gotEnc, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Equal(t, []byte(expectJSON), compactJSON(t, gotEnc))

	data, err := req.RawDataFull()
	require.NoError(t, err)
	assert.Equal(t, []byte(expectJSON), compactJSON(t, data))
}

func TestRequestReplayable(t *testing.T) {
	const message = "Nel mezzo del cammin di nostra vita mi ritrovai per una selva oscura, che' la diritta via era smarrita."
	newReplayable := func() *InvokeMethodRequest {
		return NewInvokeMethodRequest("test_method").
			WithRawDataString(message).
			WithReplay(true)
	}

	t.Run("read once", func(t *testing.T) {
		req := newReplayable()
		defer req.Close()

		require.True(t, req.CanReplay())

		t.Run("first read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("req.data is EOF", func(t *testing.T) {
			buf := make([]byte, 9)
			n, err := io.ReadFull(req.data, buf)
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.EOF)
		})

		t.Run("replay buffer is full", func(t *testing.T) {
			assert.Equal(t, len(message), req.replay.Len())
			read, err := io.ReadAll(bytes.NewReader(req.replay.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("close request", func(t *testing.T) {
			err := req.Close()
			assert.NoError(t, err)
			assert.Nil(t, req.data)
			assert.Nil(t, req.replay)
		})
	})

	t.Run("read in full three times", func(t *testing.T) {
		req := newReplayable()
		defer req.Close()

		t.Run("first read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("req.data is EOF", func(t *testing.T) {
			buf := make([]byte, 9)
			n, err := io.ReadFull(req.data, buf)
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.EOF)
		})

		t.Run("replay buffer is full", func(t *testing.T) {
			assert.Equal(t, len(message), req.replay.Len())
			read, err := io.ReadAll(bytes.NewReader(req.replay.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("second read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("third read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("close request", func(t *testing.T) {
			err := req.Close()
			assert.NoError(t, err)
			assert.Nil(t, req.data)
			assert.Nil(t, req.replay)
		})
	})

	t.Run("read in full, then partial read", func(t *testing.T) {
		req := newReplayable()
		defer req.Close()

		t.Run("first read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		r := req.RawData()
		t.Run("second, partial read", func(t *testing.T) {
			buf := make([]byte, 9)
			n, err := io.ReadFull(r, buf)
			assert.NoError(t, err)
			assert.Equal(t, 9, n)
			assert.Equal(t, message[:9], string(buf))
		})

		t.Run("read rest", func(t *testing.T) {
			read, err := io.ReadAll(r)
			assert.NoError(t, err)
			assert.Len(t, read, len(message)-9)
			// Continue from byte 9
			assert.Equal(t, message[9:], string(read))
		})

		t.Run("second read in full", func(t *testing.T) {
			read, err := req.RawDataFull()
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("close request", func(t *testing.T) {
			err := req.Close()
			assert.NoError(t, err)
			assert.Nil(t, req.data)
			assert.Nil(t, req.replay)
		})
	})

	t.Run("partial read, then read in full", func(t *testing.T) {
		req := newReplayable()
		defer req.Close()

		t.Run("first, partial read", func(t *testing.T) {
			buf := make([]byte, 9)
			n, err := io.ReadFull(req.RawData(), buf)

			assert.NoError(t, err)
			assert.Equal(t, 9, n)
			assert.Equal(t, message[:9], string(buf))
		})

		t.Run("replay buffer has partial data", func(t *testing.T) {
			assert.Equal(t, 9, req.replay.Len())
			read, err := io.ReadAll(bytes.NewReader(req.replay.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, message[:9], string(read))
		})

		t.Run("second read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("req.data is EOF", func(t *testing.T) {
			buf := make([]byte, 9)
			n, err := io.ReadFull(req.data, buf)
			assert.Equal(t, 0, n)
			assert.ErrorIs(t, err, io.EOF)
		})

		t.Run("replay buffer is full", func(t *testing.T) {
			assert.Equal(t, len(message), req.replay.Len())
			read, err := io.ReadAll(bytes.NewReader(req.replay.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("third read in full", func(t *testing.T) {
			read, err := io.ReadAll(req.RawData())
			assert.NoError(t, err)
			assert.Equal(t, message, string(read))
		})

		t.Run("close request", func(t *testing.T) {
			err := req.Close()
			assert.NoError(t, err)
			assert.Nil(t, req.data)
			assert.Nil(t, req.replay)
		})
	})

	t.Run("get ProtoWithData twice", func(t *testing.T) {
		req := newReplayable()
		defer req.Close()

		t.Run("first ProtoWithData request", func(t *testing.T) {
			pb, err := req.ProtoWithData()
			assert.NoError(t, err)
			assert.NotNil(t, pb)
			assert.NotNil(t, pb.Message)
			assert.NotNil(t, pb.Message.Data)
			assert.Equal(t, message, string(pb.Message.Data.Value))
		})

		t.Run("second ProtoWithData request", func(t *testing.T) {
			pb, err := req.ProtoWithData()
			assert.NoError(t, err)
			assert.NotNil(t, pb)
			assert.NotNil(t, pb.Message)
			assert.NotNil(t, pb.Message.Data)
			assert.Equal(t, message, string(pb.Message.Data.Value))
		})

		t.Run("close request", func(t *testing.T) {
			err := req.Close()
			assert.NoError(t, err)
			assert.Nil(t, req.data)
			assert.Nil(t, req.replay)
		})
	})
}

func compactJSON(t *testing.T, data []byte) []byte {
	out := &bytes.Buffer{}
	err := json.Compact(out, data)
	require.NoError(t, err)
	return out.Bytes()
}
