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
package wfengine

import (
	"fmt"
	"strings"

	"github.com/microsoft/durabletask-go/backend"

	"github.com/dapr/components-contrib/state"
)

const (
	inboxKeyPrefix   = "inbox"
	historyKeyPrefix = "history"
	customStatusKey  = "customStatus"
	metadataKey      = "metadata"
)

type workflowState struct {
	Inbox        []*backend.HistoryEvent
	History      []*backend.HistoryEvent
	CustomStatus string
	Generation   uint64

	// change tracking
	historyAddedCount int
	config            wfConfig
}

type workflowStateMetadata struct {
	InboxLength   int
	HistoryLength int
	Generation    uint64
}

func NewWorkflowState(config wfConfig) *workflowState {
	return &workflowState{
		Generation: 1,
		config:     config,
	}
}

func (s *workflowState) Reset() {
	s.Inbox = nil
	s.historyAddedCount = 0
	s.History = nil
	s.CustomStatus = ""
	s.Generation++
}

// ResetChangeTracking resets the change tracking counters. This should be called after a save request.
func (s *workflowState) ResetChangeTracking() {
	s.historyAddedCount = 0
}

func (s *workflowState) ApplyRuntimeStateChanges(runtimeState *backend.OrchestrationRuntimeState) {
	if runtimeState.ContinuedAsNew() {
		s.ResetChangeTracking()
		s.History = nil
	}

	newHistoryEvents := runtimeState.NewEvents()
	s.History = append(s.History, newHistoryEvents...)
	s.historyAddedCount += len(newHistoryEvents)

	s.CustomStatus = runtimeState.CustomStatus.GetValue()
}

func (s *workflowState) AddToInbox(e *backend.HistoryEvent) {
	s.Inbox = append(s.Inbox, e)
}

func (s *workflowState) ClearInbox() {
	s.Inbox = nil
}

func (s *workflowState) GetSaveRequest(actorID string, continuedAsNew bool) (req state.SetWorkflowStateRequest, err error) {
	req = state.SetWorkflowStateRequest{
		ActorID: actorID,
		Reset:   continuedAsNew,
		// TODO: Update Generation and CustomStatus only if they have changd
		Generation:   &s.Generation,
		CustomStatus: &s.CustomStatus,
	}

	// Add history
	if !continuedAsNew {
		if s.historyAddedCount > len(s.History) {
			return req, fmt.Errorf("invalid history added count: %d is greater than history's length: %d", s.historyAddedCount, len(s.History))
		}
		if s.historyAddedCount < 0 {
			return req, fmt.Errorf("invalid history added count: %d is negative", s.historyAddedCount)
		}
		req.HistoryOffset = uint64(len(s.History) - s.historyAddedCount)

		req.AppendHistory = make([][]byte, s.historyAddedCount)
		var n int
		for i := len(s.History) - s.historyAddedCount; i < len(s.History); i++ {
			req.AppendHistory[n], err = backend.MarshalHistoryEvent(s.History[i])
			if err != nil {
				return req, fmt.Errorf("failed to marshal history event %d: %w", i, err)
			}
			n++
		}
	}

	// Add inbox
	req.Inbox = make([][]byte, len(s.Inbox))
	for i := 0; i < len(s.Inbox); i++ {
		req.Inbox[i], err = backend.MarshalHistoryEvent(s.Inbox[i])
		if err != nil {
			return req, fmt.Errorf("failed to marshal inbox event %d: %w", i, err)
		}
	}

	return req, nil
}

// String implements fmt.Stringer and is primarily used for debugging purposes.
func (s *workflowState) String() string {
	if s == nil {
		return "(nil)"
	}

	inbox := make([]string, len(s.Inbox))
	for i, v := range s.Inbox {
		if v == nil {
			inbox[i] = "[(nil)]"
		} else {
			inbox[i] = "[" + v.String() + "]"
		}
	}
	history := make([]string, len(s.History))
	for i, v := range s.History {
		if v == nil {
			history[i] = "[(nil)]"
		} else {
			history[i] = "[" + v.String() + "]"
		}
	}
	return fmt.Sprintf("Inbox:%s\nHistory:%s\nCustomStatus:%s\nGeneration:%d\nhistoryAddedCount:%d\nconfig:%s",
		strings.Join(inbox, ", "), strings.Join(history, ", "),
		s.CustomStatus, s.Generation,
		s.historyAddedCount, s.config.String())
}
