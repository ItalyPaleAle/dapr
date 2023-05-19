/*
Copyright 2023 The Dapr Authors
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

package patcher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapr/dapr/pkg/injector/annotations"
)

func TestPodPatcherSetFromAnnotations(t *testing.T) {
	p := NewPodPatcher()

	// Set properties of supported kinds: bools, strings, ints
	p.SetFromAnnotations(map[string]string{
		annotations.KeyEnabled: "1",
		annotations.KeyAppID:   "myappid",
		annotations.KeyAppPort: "9876",
	})

	assert.True(t, p.Enabled)
	assert.Equal(t, "myappid", p.AppID)
	assert.Equal(t, int32(9876), p.AppPort)
}
