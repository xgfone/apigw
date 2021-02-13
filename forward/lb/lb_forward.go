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

package lb

import (
	"context"
	"time"

	"github.com/xgfone/apigw/forward"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// Backend represents the forwarded backend used by Forwarder.
//
// Notice: The Forwarder backend must implement the interface.
type Backend interface {
	loadbalancer.Endpoint
	forward.Backend
}

// Request is used to represent the request context passed to the backend.
type Request interface {
	loadbalancer.Request
	Context() *ship.Context
}

// Forwarder is a forwarder based on LB.
type Forwarder struct {
	*loadbalancer.LoadBalancer
	NewRequest func(*ship.Context) Request
	Timeout    time.Duration
}

// NewForwarder returns a new Forwarder.
func NewForwarder(maxTimeout time.Duration) *Forwarder {
	return &Forwarder{
		Timeout:      maxTimeout,
		LoadBalancer: loadbalancer.NewLoadBalancer(nil),
	}
}

// Forward implements the interface Forwarder.
func (f *Forwarder) Forward(ctx *ship.Context) (err error) {
	c := context.Background()
	if f.Timeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, f.Timeout)
		defer cancel()
	}

	var req Request
	if f.NewRequest == nil {
		req = request{c: ctx}
	} else {
		req = f.NewRequest(ctx)
	}

	if _, err = f.RoundTrip(c, req); err == loadbalancer.ErrNoAvailableEndpoint {
		err = ship.ErrBadGateway.Newf("no available backends")
	}

	return
}

type request struct {
	c *ship.Context
}

func (r request) RemoteAddrString() string { return r.c.RemoteAddr() }

func (r request) Context() *ship.Context { return r.c }
