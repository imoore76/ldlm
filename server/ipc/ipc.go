// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
This file contains the IPC method definitions and implementations
*/
package ipc

import (
	"context"
	"fmt"
	log "log/slog"

	"github.com/imoore76/ldlm/lock"
)

type (
	// IPC request and response types used in the Unlock method
	UnlockResponse bool
	UnlockRequest  struct {
		Name string
		Key  string
	}
)

// Unlock unlocks the specified lock.
//
// Parameters:
//   - UnlockRequest
//   - pointer to UnlockResponse
//
// Returns an error.
func (i *IPC) Unlock(req UnlockRequest, resp *UnlockResponse) error {

	log.Info("Handling IPC Unlock request", "name", req.Name, "key", req.Key)
	if req.Key == "" {
		for _, v := range i.lckSrv.Locks() {
			if v.Name() == req.Name {
				req.Key = v.Key()
			}
		}
	}

	if req.Key == "" {
		return lock.ErrLockDoesNotExist
	}

	// Send the request to the lock server and get the response
	unlocked, err := i.lckSrv.Unlock(context.Background(), req.Name, req.Key)
	if err != nil {
		return fmt.Errorf("failed to unlock lock: %s", err)
	}

	*resp = UnlockResponse(unlocked)
	return nil
}

type (
	// IPC request and response types for the ListLocks method
	ListLocksRequest  struct{}
	ListLocksResponse []string
)

// ListLocks handles the IPC ListLocks request.
//
// Parameters:
//   - UnlockRequest
//   - pointer to UnlockResponse
//
// Returns an error.
func (i *IPC) ListLocks(_ ListLocksRequest, locks *ListLocksResponse) error {
	log.Info("Handling IPC ListLocks request")

	for _, lk := range i.lckSrv.Locks() {
		*locks = append(*locks, fmt.Sprintf("{Name: %s, Key: %s, Size: %d}", lk.Name(), lk.Key(), lk.Size()))
	}
	return nil
}
