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
	"encoding/json"
	"io"

	"github.com/valyala/fasthttp"
)

const (
	jsonContentTypeHeader = "application/json"
	etagHeader            = "ETag"
	metadataPrefix        = "metadata."
)

// BulkGetResponse is the response object for a state bulk get operation.
type BulkGetResponse struct {
	Key      string            `json:"key"`
	Data     json.RawMessage   `json:"data,omitempty"`
	ETag     *string           `json:"etag,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// QueryResponse is the response object for querying state.
type QueryResponse struct {
	Results  []QueryItem       `json:"results"`
	Token    string            `json:"token,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// QueryItem is an object representing a single entry in query results.
type QueryItem struct {
	Key   string          `json:"key"`
	Data  json.RawMessage `json:"data"`
	ETag  *string         `json:"etag,omitempty"`
	Error string          `json:"error,omitempty"`
}

type option = func(ctx *fasthttp.RequestCtx)

// withEtag sets etag header.
func withEtag(etag *string) option {
	return func(ctx *fasthttp.RequestCtx) {
		if etag != nil {
			ctx.Response.Header.Set(etagHeader, *etag)
		}
	}
}

// withMetadata sets metadata headers.
func withMetadata(metadata map[string]string) option {
	return func(ctx *fasthttp.RequestCtx) {
		for k, v := range metadata {
			ctx.Response.Header.Set(metadataPrefix+k, v)
		}
	}
}

// withJSON overrides the content-type with application/json.
func withJSON(code int, obj []byte) option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetStatusCode(code)
		ctx.Response.SetBody(obj)
		ctx.Response.Header.SetContentType(jsonContentTypeHeader)
	}
}

// withError sets error code and jsonized error message.
func withError(code int, resp ErrorResponse) option {
	b, _ := json.Marshal(&resp)
	return withJSON(code, b)
}

// withEmpty sets 204 status code.
func withEmpty() option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetBody(nil)
		ctx.Response.SetStatusCode(fasthttp.StatusNoContent)
	}
}

// with sets a default application/json content type if content type is not present.
func with(code int, obj []byte) option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.SetStatusCode(code)
		ctx.Response.SetBody(obj)

		if len(ctx.Response.Header.ContentType()) == 0 {
			ctx.Response.Header.SetContentType(jsonContentTypeHeader)
		}
	}
}

// withStream is like "with" but accepts a stream
func withStream(code int, r io.ReadCloser) option {
	return func(ctx *fasthttp.RequestCtx) {
		ctx.Response.ImmediateHeaderFlush = true
		if len(ctx.Response.Header.ContentType()) == 0 {
			ctx.Response.Header.SetContentType(jsonContentTypeHeader)
		}
		ctx.Response.SetStatusCode(code)
		ctx.SetBodyStream(r, -1)

		/*ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			buf := make([]byte, 1024) // 1KB buffer
			var (
				nr, nw int
				err    error
				done   bool
			)
			for !done {
				nr, err = r.Read(buf)
				fmt.Println("READ", nr, err == io.EOF, err)
				if err == io.EOF {
					done = true
				} else if err != nil {
					log.Warn("Failed to read from response stream; err=%v", err)
					return
				}

				if nr > 0 {
					nw, err = w.Write(buf[:nr])
					if err != nil {
						log.Warn("Failed to write into response buffer; err=%v", err)
						return
					}
					if nr != nw {
						log.Warnf("Read %d bytes from response stream, but wrote %d bytes into response buffer", nr, nw)
						return
					}
					err = w.Flush()
					if err != nil {
						log.Warnf("Error flushing the buffer: %v", err)
					}
				}
			}
		})*/
		//ctx.Response.BodyWriteTo(ctx.Response.BodyWriter())

		//ctx.Response.SetBodyStream(r, -1)
		//ctx.Response.BodyWriteTo(ctx.Response.BodyWriter())

		/*ctx.Write([]byte{})
		time.Sleep(3 * time.Second)
		n, err := io.Copy(ctx.Response.BodyWriter(), r)
		fmt.Println("COPIED", n, err)*/

		/*w := ctx.Response.BodyWriter()
		buf := make([]byte, 4096) // 4KB buffer
		var (
			nr, nw int
			err    error
			done   bool
		)
		for !done {
			nr, err = r.Read(buf)
			fmt.Println("READ", nr, err == io.EOF, err)
			if err == io.EOF {
				done = true
			} else if err != nil {
				log.Warn("Failed to read from response stream; err=%v", err)
				return
			}

			if nr > 0 {
				nw, err = w.Write(buf[:nr])
				if err != nil {
					log.Warn("Failed to write into response buffer; err=%v", err)
					return
				}
				if nr != nw {
					log.Warnf("Read %d bytes from response stream, but wrote %d bytes into response buffer", nr, nw)
					return
				}
				//if fw, ok := w.(http.Flusher); ok {
				//	fmt.Println("FLUSHING")
				//	fw.Flush()
				//}
			}
		}

		r.Close()*/

		/*go func() {
			buf := make([]byte, 4096) // 4KB buffer
			var (
				nr   int
				err  error
				done bool
			)
			for !done {
				nr, err = r.Read(buf)
				fmt.Println("READ", nr, err == io.EOF, err)
				if err == io.EOF {
					done = true
				} else if err != nil {
					log.Warn("Failed to read from response stream; err=%v", err)
					return
				}

				if nr > 0 {
					ctx.Response.AppendBody(buf[:nr])
					//if fw, ok := w.(http.Flusher); ok {
					//	fmt.Println("FLUSHING")
					//	fw.Flush()
					//}
				}
			}

			r.Close()
		}()*/
	}
}

func respond(ctx *fasthttp.RequestCtx, options ...option) {
	for _, option := range options {
		option(ctx)
	}
}
