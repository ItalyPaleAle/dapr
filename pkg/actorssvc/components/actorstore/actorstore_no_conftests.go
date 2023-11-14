//go:build !conftests && !e2e

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

package actorstore

// StoreConfTests is empty when there's no "conftests" or "e2e" build tag.
// See actorstore_conftests.go for more information
type StoreConfTests interface {
	// Empty when there's no "conftests" build tag
}
