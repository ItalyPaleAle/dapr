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

package reminders

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReminderJSON(t *testing.T) {
	time1, _ := time.Parse(time.RFC3339, "2023-03-07T18:29:04Z")
	time2, _ := time.Parse(time.RFC3339, "2023-02-01T11:02:01Z")

	type fields struct {
		ActorID        string
		ActorType      string
		Name           string
		Data           any
		Period         string
		DueTime        time.Time
		DueTimeReq     string
		ExpirationTime time.Time
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{name: "base test", fields: fields{ActorID: "id", ActorType: "type", Name: "name"}, want: `{"actorID":"id","actorType":"type","name":"name"}`},
		{name: "with data", fields: fields{ActorID: "id", ActorType: "type", Name: "name", Data: "hi"}, want: `{"actorID":"id","actorType":"type","name":"name","data":"hi"}`},
		{name: "with period", fields: fields{ActorID: "id", ActorType: "type", Name: "name", Period: "2s"}, want: `{"period":"2s","actorID":"id","actorType":"type","name":"name"}`},
		{name: "with due time", fields: fields{ActorID: "id", ActorType: "type", Name: "name", Period: "2s", DueTimeReq: "2m", DueTime: time1}, want: `{"registeredTime":"2023-03-07T18:29:04Z","period":"2s","actorID":"id","actorType":"type","name":"name","dueTime":"2m"}`},
		{name: "with expiration time", fields: fields{ActorID: "id", ActorType: "type", Name: "name", Period: "2s", ExpirationTime: time2}, want: `{"expirationTime":"2023-02-01T11:02:01Z","period":"2s","actorID":"id","actorType":"type","name":"name"}`},
		{name: "with data as JSON object", fields: fields{ActorID: "id", ActorType: "type", Name: "name", Data: json.RawMessage(`{  "foo": [ 12, 4 ] } `)}, want: `{"actorID":"id","actorType":"type","name":"name","data":{"foo":[12,4]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			r := Reminder{
				ActorID:        tt.fields.ActorID,
				ActorType:      tt.fields.ActorType,
				Name:           tt.fields.Name,
				RegisteredTime: tt.fields.DueTime,
				DueTime:        tt.fields.DueTimeReq,
				ExpirationTime: tt.fields.ExpirationTime,
			}
			if tt.fields.Data != nil {
				if j, ok := tt.fields.Data.(json.RawMessage); ok {
					r.Data = compactJSON(t, j)
				} else {
					r.Data, _ = json.Marshal(tt.fields.Data)
				}
			}
			r.Period, err = NewReminderPeriod(tt.fields.Period)
			require.NoError(t, err)

			// Marshal
			got, err := r.MarshalJSON()
			require.NoError(t, err)

			// Compact the JSON before checking for equality
			got = compactJSON(t, got)
			assert.Equal(t, tt.want, string(got))

			// Unmarshal
			dec := Reminder{}
			err = json.Unmarshal(got, &dec)
			require.NoError(t, err)
			assert.True(t, reflect.DeepEqual(dec, r), "Got: `%#v`. Expected: `%#v`", dec, r)
		})
	}
}

func compactJSON(t *testing.T, data []byte) []byte {
	out := &bytes.Buffer{}
	err := json.Compact(out, data)
	require.NoError(t, err)
	return out.Bytes()
}
