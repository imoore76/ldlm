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

/*
This file contains the REST component that uses grpc-gateway to provide a REST gateway the the ldlm
gRPC server.
*/
package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/imoore76/ldlm/log"
	sec "github.com/imoore76/ldlm/net/security"
	pb "github.com/imoore76/ldlm/protos"
	"github.com/imoore76/ldlm/timermap"
	"google.golang.org/grpc/stats"
)

const (
	sessionCookieName = "ldlm-session"
	sessionPath       = "/session"
)

type restHandler struct {
	mux               *runtime.ServeMux
	sessions          map[string]*session
	sessionsMtx       sync.RWMutex
	grpcSrv           grpcLockServer
	timerMgr          timerManager
	sessionExpiration time.Duration
	password          string
}

// ServeHTTP serves the HTTP request.
//
// Parameters:
// - w: the http.ResponseWriter instance.
// - r: the *http.Request instance.
//
// Returns:
// - None.
func (h *restHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Validate password
	if ok := h.ValidatePassword(w, r); !ok {
		return
	}

	// Handle session requests
	if r.URL.Path == sessionPath && r.Method == http.MethodPost {
		h.CreateSession(w, r)
		return
	} else if r.URL.Path == sessionPath && r.Method == http.MethodDelete {
		h.DestroySession(w, r)
		return
	}

	// Validate session
	s, ok := h.ValidateSession(w, r)
	if !ok {
		return
	}
	defer s.mtx.Unlock()
	h.mux.ServeHTTP(w, r.WithContext(s.ctx))
}

// ValidatePassword validates the password from the HTTP request.
//
// Parameters:
// - w: the http.ResponseWriter instance.
// - r: the *http.Request instance.
// Return type: bool
func (h *restHandler) ValidatePassword(w http.ResponseWriter, r *http.Request) bool {
	isValid := func() bool {
		if h.password == "" {
			return true
		}
		auth := strings.Split(r.Header.Get("Authorization"), "Basic ")
		if len(auth) != 2 {
			return false
		}
		decoded, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			return false
		}
		password := strings.SplitN(string(decoded), ":", 2)
		if len(password) != 2 {
			return false
		}
		if password[1] == h.password {
			return true
		}
		return false
	}()

	if !isValid {
		slog.Warn(
			"Invalid password from client",
			"client_addr", r.RemoteAddr,
		)
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		w.WriteHeader(http.StatusUnauthorized) // 401
	}
	return isValid
}

// ValidateSession validates the session for the given HTTP request.
//
// Parameters:
// - w: the http.ResponseWriter instance.
// - r: the *http.Request instance.
//
// Returns:
// - *session: the session object if the session is valid.
// - bool: true if the session is valid, false otherwise.
func (h *restHandler) ValidateSession(w http.ResponseWriter, r *http.Request) (*session, bool) {
	// Check for session key in cookie
	sessionId, err := r.Cookie(sessionCookieName)

	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized) // 401
		fmt.Fprintf(w, `{"error": "session cookie not found. create a new session at %s"}`, sessionPath)
		return nil, false
	}

	// Renew the timer. If the timer has expired, the session is no longer valid
	h.sessionsMtx.Lock()
	defer h.sessionsMtx.Unlock()
	ok, err := h.timerMgr.Renew(sessionId.Value, h.sessionExpiration)
	if !ok || err != nil {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized) // 401
		fmt.Fprintf(w, `{"error": "session cookie invalid or expired. create a new session at %s"}`, sessionPath)
		return nil, false
	}
	s := h.sessions[sessionId.Value]
	s.mtx.Lock()

	http.SetCookie(w, &http.Cookie{
		Name:    sessionCookieName,
		Value:   sessionId.Value,
		Expires: time.Now().Add(h.sessionExpiration),
		Path:    "/",
	})

	return s, true
}

// DestroySession handles the destruction of a session.
//
// Parameters:
// - w: the http.ResponseWriter instance.
// - r: the *http.Request instance.
//
// Returns: None.
func (h *restHandler) DestroySession(w http.ResponseWriter, r *http.Request) {
	sessionId, err := r.Cookie(sessionCookieName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprint(w, string(s))
		return
	}
	h.sessionsMtx.Lock()
	s, ok := h.sessions[sessionId.Value]
	if !ok {
		h.sessionsMtx.Unlock()
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error": "session not found"}`)
		return
	}

	h.timerMgr.Remove(sessionId.Value)

	delete(h.sessions, sessionId.Value)
	h.sessionsMtx.Unlock()
	s.mtx.Lock()
	defer s.mtx.Unlock()

	h.grpcSrv.HandleConn(s.ctx, &stats.ConnEnd{})

	http.SetCookie(w, &http.Cookie{
		Name:    sessionCookieName,
		Value:   "",
		Expires: time.Time{},
		Path:    "/",
	})

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"session_id": ""}`)
}

// CreateSession creates a new session and adds it to the sessions map.
//
// Parameters:
// - w: the http.ResponseWriter instance.
// - r: the *http.Request instance.
//
// Returns: None.
func (h *restHandler) CreateSession(w http.ResponseWriter, r *http.Request) {

	// Create new session
	sessionId := strings.ReplaceAll(uuid.NewString(), "-", "")

	var ip string
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx == -1 {
		ip = "0.0.0.0"
	} else {
		ip = r.RemoteAddr[:idx]
	}
	ctx := h.grpcSrv.TagConn(r.Context(), &stats.ConnTagInfo{
		RemoteAddr: &net.TCPAddr{
			IP:   net.ParseIP(ip),
			Port: 0,
		},
	})

	cxLogger := log.FromContextOrDefault(ctx)
	cxLogger = cxLogger.With("rest_session_id", sessionId)
	ctx = log.ToContext(cxLogger, ctx)

	h.sessionsMtx.Lock()
	h.sessions[sessionId] = &session{
		ctx: ctx,
		mtx: sync.Mutex{},
	}

	// Add timer to expire session
	h.timerMgr.Add(
		sessionId,
		h.onTimeoutFunc(sessionId),
		h.sessionExpiration,
	)
	h.sessionsMtx.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:    sessionCookieName,
		Value:   sessionId,
		Expires: time.Now().Add(h.sessionExpiration),
		Path:    "/",
	})
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201
	fmt.Fprintf(w, `{"session_id": "%s"}`, sessionId)
}

// onTimeoutFunc returns a function that can be used to remove a session from the sessions map.
func (h *restHandler) onTimeoutFunc(sessionId string) func() {
	return func() {

		h.sessionsMtx.Lock()
		s, ok := h.sessions[sessionId]
		if !ok {
			h.sessionsMtx.Unlock()
			return
		}
		delete(h.sessions, sessionId)
		ctxLog := log.FromContextOrDefault(s.ctx)
		ctxLog.Info(
			"REST session timeout",
			"rest_session_id", sessionId,
			"idle", h.sessionExpiration,
		)
		defer s.mtx.Unlock()
		s.mtx.Lock()
		h.sessionsMtx.Unlock()

		h.grpcSrv.HandleConn(s.ctx, &stats.ConnEnd{})
	}
}

// Run starts a REST server that handles LDLM gRPC requests.
//
// Parameters:
// - server: The gRPC server that handles LDLM requests.
// - conf: The configuration for the REST server.
// - sConf: The security configuration for the REST server.
//
// Returns:
// - A function that can be used to stop the server.
// - An error if there was a problem starting the server.
func Run(server grpcLockServer, conf *RestConfig, sConf *sec.SecurityConfig) (func(), error) {

	gwServer, closer, err := NewRestServer(server, conf, sConf)
	if err != nil {
		return nil, err
	}

	go func() {
		if sConf.TlsCert != "" && sConf.TlsKey != "" {
			if err := gwServer.ListenAndServeTLS(sConf.TlsCert, sConf.TlsKey); err != nil && err != http.ErrServerClosed {
				panic(err)
			}
		} else if err := gwServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	slog.Warn("REST server started. Listening on " + conf.RestListenAddress)
	return closer, nil
}

// NewRestServer creates a new REST server that handles LDLM gRPC requests.
//
// Parameters:
// - server: The gRPC server that handles LDLM requests.
// - conf: The configuration for the REST server.
// - sConf: The security configuration for the REST server.
//
// Returns:
// - *http.Server: The newly created REST server.
// - func(): A function to close the server.
// - error: An error if there was a problem creating the server.
func NewRestServer(server grpcLockServer, conf *RestConfig, sConf *sec.SecurityConfig) (*http.Server, func(), error) {

	// Register Handler
	gwMux := runtime.NewServeMux()
	if err := pb.RegisterLDLMHandlerServer(context.Background(), gwMux, server); err != nil {
		return nil, nil, err
	}

	tm, tmCloser := timermap.New()

	handler := &restHandler{
		mux:               gwMux,
		sessions:          make(map[string]*session),
		sessionsMtx:       sync.RWMutex{},
		grpcSrv:           server,
		timerMgr:          tm,
		sessionExpiration: conf.RestSessionTimeout,
		password:          sConf.Password,
	}

	tls, err := sec.GetTLSConfig(sConf)
	if err != nil {
		return nil, nil, err
	}

	// Create and return the REST server
	srv := &http.Server{
		TLSConfig: tls,
		Addr:      conf.RestListenAddress,
		Handler:   handler,
	}
	return srv, func() {
		tmCloser()
		srv.Close()
	}, nil
}
