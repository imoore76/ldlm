// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	config "github.com/imoore76/configurature"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/stats"

	sec "github.com/imoore76/ldlm/net/security"
	pb "github.com/imoore76/ldlm/protos"
)

func TestSessionCreate(t *testing.T) {
	assert := assert.New(t)
	gs := &testGrpcServer{}
	rc := getRestConf(nil)
	s, close, err := NewRestServer(gs, rc, getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)
	assert.Equal("application/json", rec.Header().Get("Content-Type"), "expected content type %s, got %s", "application/json", rec.Header().Get("Content-Type"))
	assert.Len(rec.Result().Cookies(), 1, "expected 1 cookie, got %d", len(rec.Result().Cookies()))

	c := rec.Result().Cookies()[0]
	assert.Equal(sessionCookieName, c.Name, "expected cookie name %s, got %s", sessionCookieName, c.Name)
	assert.Equal("/", c.Path, "expected cookie path %s, got %s", "/", c.Path)
	expires := time.Since(c.Expires).Round(time.Minute).Abs()
	assert.Equal(rc.RestSessionTimeout, expires, "expected cookie expires %s, got %s", rc.RestSessionTimeout, expires)

	assert.NotNil(gs.tagConnCall, "expected tagConnCall not to be nil")

}

func TestSessionDestroy(t *testing.T) {
	assert := assert.New(t)
	gs := &testGrpcServer{}
	rc := getRestConf(nil)
	s, close, err := NewRestServer(gs, rc, getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("POST", sessionPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)
	assert.Equal("application/json", rec.Header().Get("Content-Type"), "expected content type %s, got %s", "application/json", rec.Header().Get("Content-Type"))
	assert.Len(rec.Result().Cookies(), 1, "expected 1 cookie, got %d", len(rec.Result().Cookies()))

	c := rec.Result().Cookies()[0]

	rec = httptest.NewRecorder()
	// Create a new http.Request.
	req, err = http.NewRequest("DELETE", sessionPath, nil)
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}

	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code, "expected status code %d, got %d", http.StatusOK, rec.Code)
	assert.Equal(`{"session_id": ""}`, rec.Body.String(), "expected body %s, got %s", `{"session_id": ""}`, rec.Body.String())
	assert.Len(rec.Result().Cookies(), 1, "expected 0 cookies, got %d", len(rec.Result().Cookies()))

	c = rec.Result().Cookies()[0]
	assert.Equal(sessionCookieName, c.Name, "expected cookie name %s, got %s", sessionCookieName, c.Name)
	assert.Equal("", c.Value, "expected cookie value %s, got %s", "", c.Value)

	assert.NotNil(gs.handleConnCall, "expected handleConnCall not to be nil")
	assert.IsType(&stats.ConnEnd{}, gs.handleConnCall, "expected handleConnCall to be of type *stats.ConnEnd")

}

func TestSessionDestroy_InvalidSession(t *testing.T) {
	assert := assert.New(t)
	gs := &testGrpcServer{}
	rc := getRestConf(nil)
	s, close, err := NewRestServer(gs, rc, getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("DELETE", sessionPath, nil)
	req.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "invalid",
	})
	if err != nil {
		t.Fatal(err)
	}

	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusConflict, rec.Code, "expected status code %d, got %d", http.StatusConflict, rec.Code)
}

func TestSessionTimeout_DoesntTimeout(t *testing.T) {

	assert := assert.New(t)
	gs := &testGrpcServer{}
	rc := getRestConf(nil)
	s, close, err := NewRestServer(gs, rc, getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)

	c := rec.Result().Cookies()[0]

	time.Sleep(2 * time.Second)

	rec = httptest.NewRecorder()

	// Create a new http.Request.
	req, err = http.NewRequest("POST", "/v1/lock", strings.NewReader(`{"name":"testlock"}`))
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code, "expected status code %d, got %d", http.StatusOK, rec.Code)

}

func TestSessionTimeout(t *testing.T) {

	assert := assert.New(t)
	gs := &testGrpcServer{}
	rc := getRestConf(nil)
	rc.RestSessionTimeout = 1 * time.Second
	s, close, err := NewRestServer(gs, rc, getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)

	c := rec.Result().Cookies()[0]

	time.Sleep(2 * time.Second)

	rec = httptest.NewRecorder()

	// Create a new http.Request.
	req, err = http.NewRequest("POST", "/v1/lock", strings.NewReader(`{"name":"testlock"}`))
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusUnauthorized, rec.Code, "expected status code %d, got %d", http.StatusUnauthorized, rec.Code)

	assert.NotNil(gs.handleConnCall, "expected handleConnCall not to be nil")
	assert.IsType(&stats.ConnEnd{}, gs.handleConnCall, "expected handleConnCall to be of type *stats.ConnEnd")

}

func TestInvalidPassword(t *testing.T) {
	assert := assert.New(t)
	gs := &testGrpcServer{}
	sc := getSecConf(nil)
	sc.Password = "foo"
	s, close, err := NewRestServer(gs, getRestConf(nil), sc)
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	// Create a new http.Request.
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusUnauthorized, rec.Code, "expected status code %d, got %d", http.StatusUnauthorized, rec.Code)
	assert.Equal(`Basic realm="Restricted"`, rec.Header().Get("WWW-Authenticate"), "expected WWW-Authenticate %s, got %s", `Basic realm="Restricted"`, rec.Header().Get("WWW-Authenticate"))

}

func TestValidPassword(t *testing.T) {
	assert := assert.New(t)
	gs := &testGrpcServer{}
	sc := getSecConf(nil)
	sc.Password = "::fooasdf;oij:asdjfoiajdfj"
	s, close, err := NewRestServer(gs, getRestConf(nil), sc)
	if err != nil {
		panic(err)
	}
	defer close()

	rec := httptest.NewRecorder()

	auth := base64.StdEncoding.EncodeToString([]byte(":" + sc.Password))

	// Create a new http.Request.
	req, err := http.NewRequest("POST", "/session", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)
	c := rec.Result().Cookies()[0]

	rec = httptest.NewRecorder()

	// Create a new http.Request.
	req, err = http.NewRequest("POST", "/v1/lock", strings.NewReader(`{"name":"testlock"}`))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code, "expected status code %d, got %d", http.StatusOK, rec.Code)

}

func TestGrpcCall(t *testing.T) {

	assert := assert.New(t)
	gs := &testGrpcServer{}
	s, close, err := NewRestServer(gs, getRestConf(nil), getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	// Create a new http.Request.
	rec := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)

	c := rec.Result().Cookies()[0]
	// Lock request
	// Create a new http.Request.
	rec = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/v1/lock", strings.NewReader(`{"name":"testlock"}`))
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code, "expected status code %d, got %d", http.StatusOK, rec.Code)
	assert.Equal("application/json", rec.Header().Get("Content-Type"), "expected content type %s, got %s", "application/json", rec.Header().Get("Content-Type"))

	lock := struct {
		Name   string `json:"name"`
		Key    string `json:"key"`
		Locked bool   `json:"locked"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{}
	json.Unmarshal(rec.Body.Bytes(), &lock)

	assert.Equal(lock.Name, "testlock", "expected lock.Name to be %s, got %s", "testlock", lock.Name)
	assert.Equal(lock.Key, "static key", "expected lock.Key to be %s, got %s", "testlock", lock.Key)
	assert.Nil(lock.Error, "expected lock.Error to be nil, got %v", lock.Error)
	assert.True(lock.Locked, "expected lock.Locked to be true, got %v", lock.Locked)

	assert.IsType(gs.tLockCall, &pb.TryLockRequest{}, "expected tLockCall to be of type *pb.TryLockRequest")
	assert.Equal(gs.tLockCall.Name, "testlock", "expected tLockCall.Name to be %s, got %s", "testlock", gs.tLockCall.Name)
	assert.Nil(gs.tLockCall.LockTimeoutSeconds, "expected tLockCall.LockTimeoutSeconds to be nil, got %v", gs.tLockCall.LockTimeoutSeconds)
}

func TestGrpcCall_Error(t *testing.T) {

	assert := assert.New(t)
	gs := &testGrpcServer{
		tLockRes: &pb.LockResponse{
			Error:  &pb.Error{Code: 1, Message: "test error"},
			Key:    "",
			Name:   "testlock",
			Locked: false,
		},
	}
	s, close, err := NewRestServer(gs, getRestConf(nil), getSecConf(nil))
	if err != nil {
		panic(err)
	}
	defer close()

	// Create a new http.Request.
	rec := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusCreated, rec.Code, "expected status code %d, got %d", http.StatusCreated, rec.Code)

	c := rec.Result().Cookies()[0]
	// Lock request
	// Create a new http.Request.
	rec = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/v1/lock", strings.NewReader(`{"name":"testlock"}`))
	req.AddCookie(c)
	if err != nil {
		t.Fatal(err)
	}
	s.Handler.ServeHTTP(rec, req)

	assert.Equal(http.StatusOK, rec.Code, "expected status code %d, got %d", http.StatusOK, rec.Code)
	assert.Equal("application/json", rec.Header().Get("Content-Type"), "expected content type %s, got %s", "application/json", rec.Header().Get("Content-Type"))

	lock := struct {
		Name   string
		Key    string
		Locked bool
		Error  *struct {
			Code    string
			Message string
		}
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &lock); err != nil {
		t.Fatal(err)
	}

	fmt.Println(rec.Body.String())
	assert.Equal(lock.Name, "testlock", "expected lock.Name to be %s, got %s", "testlock", lock.Name)
	assert.Equal(lock.Key, "", "expected lock.Key to be %s, got %s", "testlock", lock.Key)
	assert.False(lock.Locked, "expected lock.Locked to be false, got %v", lock.Locked)
	assert.NotNil(lock.Error, "expected lock.Error to not be nil, got %v", lock.Error)

	assert.Equal(lock.Error.Code, "LockDoesNotExist", "expected lock.Error.Code to be %s, got %s", "LockDoesNotExist", lock.Error.Code)
	assert.Equal(lock.Error.Message, "test error", "expected lock.Error.Message to be %s, got %s", "test error", lock.Error.Message)

	assert.IsType(gs.tLockCall, &pb.TryLockRequest{}, "expected tLockCall to be of type *pb.TryLockRequest")
	assert.Equal(gs.tLockCall.Name, "testlock", "expected tLockCall.Name to be %s, got %s", "testlock", gs.tLockCall.Name)
	assert.Nil(gs.tLockCall.LockTimeoutSeconds, "expected tLockCall.LockTimeoutSeconds to be nil, got %v", gs.tLockCall.LockTimeoutSeconds)
}

type testGrpcServer struct {
	rLockRes *pb.LockResponse
	tLockRes *pb.LockResponse
	uLockRes *pb.UnlockResponse

	rLockCall *pb.RenewRequest
	tLockCall *pb.TryLockRequest
	uLockCall *pb.UnlockRequest

	handleConnCall stats.ConnStats
	tagConnCall    *stats.ConnTagInfo
	pb.UnimplementedLDLMServer
}

func (g *testGrpcServer) HandleConn(ctx context.Context, s stats.ConnStats) {
	g.handleConnCall = s
}
func (g *testGrpcServer) Lock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	return &pb.LockResponse{}, nil
}
func (g *testGrpcServer) Renew(ctx context.Context, req *pb.RenewRequest) (*pb.LockResponse, error) {
	g.rLockCall = req
	return g.rLockRes, nil
}
func (g *testGrpcServer) TryLock(ctx context.Context, req *pb.TryLockRequest) (*pb.LockResponse, error) {
	g.tLockCall = req

	if g.tLockRes == nil {
		return &pb.LockResponse{Name: req.Name, Key: "static key", Locked: true}, nil
	}
	return g.tLockRes, nil
}
func (g *testGrpcServer) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	g.uLockCall = req
	return g.uLockRes, nil
}
func (g *testGrpcServer) TagConn(ctx context.Context, s *stats.ConnTagInfo) context.Context {
	g.tagConnCall = s
	return ctx
}
func (g *testGrpcServer) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}
func (g *testGrpcServer) HandleRPC(ctx context.Context, _ stats.RPCStats) {}

func getRestConf(args []string) *RestConfig {
	return config.Configure[RestConfig](&config.Options{
		Args: args,
	})
}

func getSecConf(args []string) *sec.SecurityConfig {
	return config.Configure[sec.SecurityConfig](&config.Options{
		Args: args,
	})
}
