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

package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/imoore76/go-ldlm/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func lockRefresher(c pb.LDLMClient, ctx context.Context, name string, key string, timeout_seconds int32) func() {

	interval := max(timeout_seconds-30, 10)
	stopCh := make(chan struct{}, 1)
	go func() {
		defer fmt.Println("Stopped refreshing lock")
		for {
			select {
			case <-time.After(time.Duration(interval) * time.Second):
				r, err := c.RefreshLock(ctx, &pb.RefreshLockRequest{
					Name:               name,
					Key:                key,
					LockTimeoutSeconds: timeout_seconds,
				})
				if err != nil {
					fmt.Printf("Error refreshing lock: %v\n", err)
					return
				}
				if !r.Locked {
					fmt.Printf("Could not refresh lock: %v\n", r.Error)
					return
				}
			case <-stopCh:
				return
			}

		}
	}()
	return func() {
		fmt.Println("Stopping refreshing lock")
		stopCh <- struct{}{}
	}
}

func main() {
	conn, err := grpc.NewClient(
		"localhost:3144",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	client := pb.NewLDLMClient(conn)

	var lockTimeout = int32(59)
	r, err := client.Lock(ctx, &pb.LockRequest{
		Name:               "work-item1",
		LockTimeoutSeconds: &lockTimeout,
	})
	if err != nil {
		panic(err)
	}
	if r.Error != nil && r.Error.Code == pb.ErrorCode_LockWaitTimeout {
		panic("timed out waiting for lock")
	}
	if !r.Locked {
		panic(fmt.Sprintf("Could not lock work-item1: %v", r.Error))
	}

	closer := lockRefresher(client, ctx, "work-item1", r.Key, lockTimeout)
	defer closer()

	defer client.Unlock(ctx, &pb.UnlockRequest{
		Name: "work-item1",
		Key:  r.Key,
	})

	fmt.Println("Starting work...")

	time.Sleep(31 * time.Second)

	fmt.Println("Work complete")
}
