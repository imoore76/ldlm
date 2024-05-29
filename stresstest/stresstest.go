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
stresstest exposes a StressTest() function which can be used to stress test
ldlm. It's particularly useful when checking for race conditions. It's meant
to be used by ldlm developers and is not exposed via -help. To run:

	ldlm-stress -help
*/
package stresstest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	log "log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/imoore76/go-ldlm/protos"
)

var (
	lockNames     = []string{}
	lockNamesLock = sync.RWMutex{}
	ctx           = context.Background()
	checkerLock   = sync.RWMutex{}
	NoTimeout     = int32(0)
)

type Config struct {
	SleepForMaxSeconds int32         `desc:"Sleep for this many seconds (max) between operations. Actual duration will be random between 0 and this value." default:"10" short:"s"`
	NumClients         int32         `desc:"Number of clients to run" default:"80" short:"c"`
	NumLocks           int32         `desc:"Number of locks to create" default:"24" short:"l"`
	Address            string        `desc:"Address (host:port) at which the ldlm server is listening" default:"localhost:3144" short:"a"`
	RandomizeNames     bool          `desc:"Randomize lock names" default:"true" short:"r"`
	RandomizeInterval  time.Duration `desc:"Interval at which to randomize lock names" default:"7s" short:"i"`
	UseTls             bool          `desc:"Use SSL" default:"false"`
	SkipVerify         bool          `desc:"Skip SSL verification" default:"false"`
	CAFile             string        `desc:"File containing client CA certificate." default:""`
	TlsCert            string        `desc:"File containing TLS certificate" default:""`
	TlsKey             string        `desc:"File containing TLS key" default:""`
	Password           string        `desc:"Password to send" default:""`
}

type apiClient struct {
	pbc         pb.LDLMClient
	ClientId    string
	Name        string
	LastLock    time.Time
	SleepForMax int32
}

func newApiClient(conf *Config) *apiClient {
	creds := insecure.NewCredentials()
	if conf.UseTls || conf.TlsCert != "" {
		tlsc := &tls.Config{
			ServerName:         strings.Split(conf.Address, ":")[0],
			InsecureSkipVerify: conf.SkipVerify,
		}
		if conf.TlsCert != "" {
			clientCert, err := tls.LoadX509KeyPair(conf.TlsCert, conf.TlsKey)
			if err != nil {
				panic("LoadX509KeyPair(): " + err.Error())
			}
			tlsc.Certificates = []tls.Certificate{clientCert}
		}
		if conf.CAFile != "" {
			if cacert, err := os.ReadFile(conf.CAFile); err != nil {
				panic("Failed to read CA certificate: " + err.Error())
			} else {
				certPool := x509.NewCertPool()
				if !certPool.AppendCertsFromPEM(cacert) {
					panic("failed to add CA certificate")
				}
				tlsc.RootCAs = certPool
			}
		}
		creds = credentials.NewTLS(tlsc)
	}

	conn, err := grpc.Dial(
		conf.Address,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		panic(err)
	}

	cid, _ := uuid.NewRandom()
	return &apiClient{
		pbc:         pb.NewLDLMClient(conn),
		ClientId:    cid.String(),
		LastLock:    time.Now(),
		SleepForMax: conf.SleepForMaxSeconds,
	}

}

func randomWaitLock(c *apiClient) {
	checkerLock.RLock()
	defer checkerLock.RUnlock()
	lockNameIdx := rand.IntN(len(lockNames))
	lockNamesLock.RLock()
	lockName := lockNames[lockNameIdx]
	lockNamesLock.RUnlock()

	log.Info(fmt.Sprintf("Client %s waiting to lock %s", c.ClientId, lockName))
	req := &pb.LockRequest{
		Name: lockName}

	if rand.Int32N(100) > 50 {
		tmr := int32(rand.Int32N(c.SleepForMax))
		req.WaitTimeoutSeconds = &tmr
	}
	if rand.Int32N(100) > 50 {
		to := int32(rand.Int32N(c.SleepForMax))
		req.LockTimeoutSeconds = &to
	}
	r, err := c.pbc.Lock(ctx, req)
	if err != nil {
		panic(err)
	}

	if !r.Locked {
		log.Info(fmt.Sprintf("Client %s couldn't lock %s.", c.ClientId, lockName))
		return
	}
	c.LastLock = time.Now()
	c.Name = lockName
	sleepFor := rand.Int32N(c.SleepForMax * 1000)
	log.Info(fmt.Sprintf("Client %s locked %s. Waiting %d milliseconds.", c.ClientId, lockName, sleepFor))
	time.Sleep(time.Duration(sleepFor) * time.Millisecond)

	_, err = c.pbc.Unlock(ctx, &pb.UnlockRequest{Name: lockName, Key: r.Key})
	c.Name = ""
	if err != nil {
		errStr := fmt.Sprintf("Client %s error unlocking %s err: %v.", c.ClientId, lockName, err)
		panic(errStr)
	}
	log.Info(fmt.Sprintf("Client %s unlocked %s.", c.ClientId, lockName))
}

func randomTryLock(c *apiClient) {
	checkerLock.RLock()
	defer checkerLock.RUnlock()

	lockNameIdx := rand.IntN(len(lockNames))

	lockNamesLock.RLock()
	lockName := lockNames[lockNameIdx]
	lockNamesLock.RUnlock()

	req := &pb.TryLockRequest{
		Name: lockName,
	}
	if rand.Int32N(100) > 50 {
		to := int32(rand.Int32N(c.SleepForMax))
		req.LockTimeoutSeconds = &to
	}

	r, err := c.pbc.TryLock(ctx, req)
	if err != nil {
		panic(err)
	}

	if !r.Locked {
		log.Info(fmt.Sprintf("Client %s didn't trylock %s.", c.ClientId, lockName))
		return
	}
	c.LastLock = time.Now()
	c.Name = lockName
	sleepFor := rand.Int32N(c.SleepForMax * 1000)
	log.Info(fmt.Sprintf("Client %s locked %s. Waiting %d milliseconds.", c.ClientId, lockName, sleepFor))
	time.Sleep(time.Duration(sleepFor) * time.Millisecond)

	_, err = c.pbc.Unlock(ctx, &pb.UnlockRequest{Name: lockName, Key: r.Key})
	c.Name = ""
	if err != nil {
		errStr := fmt.Sprintf("Client %s error unlocking %s err: %v.", c.ClientId, lockName, err)
		panic(errStr)
	}
	log.Info(fmt.Sprintf("Client %s unlocked %s.", c.ClientId, lockName))

}

func Run(conf *Config) {
	if conf.Password != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", conf.Password)
	}

	fnList := []func(c *apiClient){
		randomTryLock,
		randomWaitLock,
	}

	for range conf.NumLocks {
		lockNames = append(lockNames, uuid.NewString())
	}

	if conf.RandomizeNames {
		go swapNames(conf.RandomizeInterval)
	}

	clients := []*apiClient{}
	for range conf.NumClients {
		go func() {
			c := newApiClient(conf)
			clients = append(clients, c)
			for {
				fnIdx := rand.IntN(len(fnList))
				fnList[fnIdx](c)
			}
		}()
	}
	for {
		time.Sleep(time.Duration(5) * time.Second)
		seen := make(map[string]struct{})
		checkerLock.Lock()
		for _, c := range clients {
			if c.Name == "" {
				continue
			}
			if _, ok := seen[c.Name]; ok {
				panic(fmt.Sprintf("two locks for %s", c.Name))
			}
			seen[c.Name] = struct{}{}
			if time.Since(c.LastLock) > (5 * time.Minute) {
				panic("Time since last lock is > 5 minutes")
			}
		}
		checkerLock.Unlock()

	}
}

func swapNames(interval time.Duration) {
	for {
		u, _ := uuid.NewRandom()
		newName := u.String()
		lockNameIdx := rand.IntN(len(lockNames))
		lockNamesLock.Lock()
		lockNames[lockNameIdx] = newName
		lockNamesLock.Unlock()
		time.Sleep(interval)
	}
}
