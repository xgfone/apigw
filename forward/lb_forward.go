// Copyright 2021 xgfone
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

package forward

import (
	"context"
	"time"

	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// HTTPRequest is an adapter from http.Request to loadbalancer.Request.
type HTTPRequest struct {
	CookieName  string
	SessionName string
	*ship.Context
}

// NewHTTPRequest returns a new HTTPRequest.
func NewHTTPRequest(ctx *ship.Context, cookieName, sessionName string) HTTPRequest {
	return HTTPRequest{CookieName: cookieName, SessionName: sessionName, Context: ctx}
}

// RemoteAddrString implements the interface loadbalancer.Request.
func (r HTTPRequest) RemoteAddrString() string { return r.RemoteAddr() }

// SessionID implements the interface loadbalancer.RequestSession.
func (r HTTPRequest) SessionID() string {
	if r.CookieName != "" {
		if c := r.Cookie(r.CookieName); c != nil {
			return c.Value
		}
	}

	if r.SessionName != "" {
		return r.GetHeader(r.SessionName)
	}

	return ""
}

// LBForwarder is a forwarder based on LB.
type LBForwarder struct {
	*loadbalancer.LoadBalancer
	Timeout     time.Duration
	CookieName  string
	SessionName string
}

// NewLBForwarder returns a new LBForwarder.
func NewLBForwarder(maxTimeout time.Duration) *LBForwarder {
	return &LBForwarder{
		Timeout:      maxTimeout,
		LoadBalancer: loadbalancer.NewLoadBalancer(nil),
	}
}

// Forward implements the interface Forwarder.
func (f *LBForwarder) Forward(ctx *ship.Context) (err error) {
	c := context.Background()
	if f.Timeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, f.Timeout)
		defer cancel()
	}

	_, err = f.RoundTrip(c, NewHTTPRequest(ctx, f.CookieName, f.SessionName))
	return
}
