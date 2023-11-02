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

package server

import (
	"time"

	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

type connectedHosts map[string]connectedHostInfo

type connectedHostInfo struct {
	serverMsgCh chan actorsv1pb.ServerStreamMessage
	actorTypes  []string
	pausedUntil time.Time
	address     string
}

func (ch connectedHosts) updateCachedData(curConnectedHostsIDsLen, curActorTypeResLen int) (connectedHostsIDs, actorTypesRes []string) {
	// For connectedHostsIDs, allocate an initial capacity for the number of hosts
	connectedHostsIDs = make([]string, 0, len(ch))

	// For actorTypeResLen, allocate an initial capacity equal to the current capacity + 2 for each additional host that was added
	addedHosts := len(ch) - curConnectedHostsIDsLen
	if addedHosts < 0 {
		addedHosts = 0
	}
	actorTypesRes = make([]string, 0, curConnectedHostsIDsLen+addedHosts*2)

	foundTypes := make(map[string]struct{}, cap(actorTypesRes))
	now := time.Now()
	for name, info := range ch {
		// If the host is paused, do not add it to the lists
		if info.pausedUntil.After(now) {
			continue
		}

		// Add the host ID
		connectedHostsIDs = append(connectedHostsIDs, name)

		// Add the actor types avoiding duplicates
		for _, at := range info.actorTypes {
			_, ok := foundTypes[at]
			if ok {
				continue
			}
			foundTypes[at] = struct{}{}
			actorTypesRes = append(actorTypesRes, at)
		}
	}
	return connectedHostsIDs, actorTypesRes
}

// Adds or updates a connected host.
func (s *server) setConnectedHost(actorHostID string, info connectedHostInfo) {
	// Set in the connectedHosts map then update the cached actor types
	s.connectedHostsLock.Lock()
	s.connectedHosts[actorHostID] = info
	s.connectedHostsIDs, s.connectedHostsActorTypes = s.connectedHosts.updateCachedData(len(s.connectedHostsIDs), len(s.connectedHostsActorTypes))
	s.connectedHostsLock.Unlock()
}

// Removes a connected host.
func (s *server) removeConnectedHost(actorHostID string) {
	s.connectedHostsLock.Lock()
	delete(s.connectedHosts, actorHostID)
	s.connectedHostsIDs, s.connectedHostsActorTypes = s.connectedHosts.updateCachedData(len(s.connectedHostsIDs), len(s.connectedHostsActorTypes))
	s.connectedHostsLock.Unlock()
}

// Updates the cached connected hosts data.
func (s *server) updateConnectedHostCache() {
	s.connectedHostsLock.Lock()
	s.connectedHostsIDs, s.connectedHostsActorTypes = s.connectedHosts.updateCachedData(len(s.connectedHostsIDs), len(s.connectedHostsActorTypes))
	s.connectedHostsLock.Unlock()
}

// Returns true if the given address belongs to an actor host that is currently connected to this instance.
func (s *server) isHostAddressConnected(address string) bool {
	if address == "" {
		return false
	}

	s.connectedHostsLock.RLock()
	defer s.connectedHostsLock.RUnlock()
	for id := range s.connectedHosts {
		if s.connectedHosts[id].address == address {
			return true
		}
	}
	return false
}

// Returns true if the actor host is currently paused.
// If the host isn't in the cache, returns true.
func (s *server) isConnectedHostPaused(actorHostID string) bool { //nolint:unused
	s.connectedHostsLock.RLock()
	host, ok := s.connectedHosts[actorHostID]
	s.connectedHostsLock.RUnlock()
	return !ok || host.pausedUntil.After(time.Now())
}
