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

	"github.com/xgfone/apigw"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// Forwarder is a forwarder based on LB.
type Forwarder struct {
	*loadbalancer.HealthCheck
	*loadbalancer.LoadBalancer
	NewRequest func(*apigw.Context) Request
	Timeout    time.Duration
}

// NewForwarder returns a new Forwarder.
//
// In order to implement the function of the session stick, you maybe set
// Forwarder.LoadBalancer.Session to a session manager, such as
//   Forwarder.LoadBalancer.Session = loadbalancer.NewMemorySessionManager()
func NewForwarder(name string, maxTimeout time.Duration) *Forwarder {
	lb := loadbalancer.NewLoadBalancer(nil)
	lb.Name = name
	return &Forwarder{LoadBalancer: lb, Timeout: maxTimeout}
}

// Name returns the name of the forwarder and implements the interface
// loadbalancer.Updater.
func (f *Forwarder) Name() string { return f.LoadBalancer.Name }

// Endpoints returns all the underlying endpoints.
func (f *Forwarder) Endpoints() loadbalancer.Endpoints {
	return f.EndpointManager().Endpoints()
}

// AddEndpoint implements the interface loadbalancer.Updater.
func (f *Forwarder) AddEndpoint(ep loadbalancer.Endpoint) {
	f.EndpointManager().AddEndpoint(ep)
}

// DelEndpoint implements the interface loadbalancer.Updater.
func (f *Forwarder) DelEndpoint(ep loadbalancer.Endpoint) {
	f.EndpointManager().DelEndpoint(ep)
}

// Close cleans and releases the underlying resource.
func (f *Forwarder) Close() error {
	for _, ep := range f.Endpoints() {
		if gb, ok := loadbalancer.UnwrapEndpoint(ep).(GroupBackend); ok {
			gb.BackendGroup().DelUpdater(f)
		}
	}

	if f.HealthCheck != nil {
		f.HealthCheck.DelEndpointsByUpdater(f)
		f.HealthCheck.UnsubscribeByUpdater(f)
	}

	return f.LoadBalancer.Close()
}

// AddBackends adds a set of Backends.
func (f *Forwarder) AddBackends(backends []Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.AddBackend(backends[i])
	}
}

// DelBackends deletes a set of Backends.
func (f *Forwarder) DelBackends(backends []Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.DelBackend(backends[i])
	}
}

// AddBackend adds the backend.
func (f *Forwarder) AddBackend(b Backend) {
	if gb, ok := loadbalancer.UnwrapEndpoint(b).(GroupBackend); ok {
		gb.BackendGroup().AddUpdater(f)
		return
	}

	if f.HealthCheck == nil {
		f.AddEndpoint(b)
		return
	}

	hc := b.HealthCheck()
	if hc.Interval == 0 {
		hc.Interval = f.HealthCheck.Interval
	}
	if hc.Timeout == 0 {
		hc.Timeout = f.HealthCheck.Timeout
	}
	if hc.RetryNum == 0 {
		hc.RetryNum = f.HealthCheck.RetryNum
	}

	f.HealthCheck.Subscribe(b.String(), f)
	f.HealthCheck.AddEndpointWithDuration(b, hc.Interval, hc.Timeout, hc.RetryNum)
}

// DelBackend deletes the backend.
func (f *Forwarder) DelBackend(b Backend) {
	if gb, ok := loadbalancer.UnwrapEndpoint(b).(GroupBackend); ok {
		gb.BackendGroup().DelUpdater(f)
		return
	}

	if f.HealthCheck != nil {
		f.HealthCheck.Unsubscribe(b.String())
		f.HealthCheck.DelEndpoint(b)
	}
	f.DelEndpoint(b)
}

// Backends returns all the backends.
func (f *Forwarder) Backends() []Backend {
	var eps loadbalancer.Endpoints
	online := func(ep loadbalancer.Endpoint) bool { return true }
	if f.HealthCheck == nil {
		eps = f.EndpointManager().Endpoints()
	} else {
		eps = f.HealthCheck.Endpoints()
		online = func(ep loadbalancer.Endpoint) bool {
			return ep.IsHealthy(context.Background())
		}
	}

	bs := make([]Backend, len(eps))
	for i, _len := 0, len(eps); i < _len; i++ {
		bs[i] = newEndpointBackend(f.Provider, eps[i], online(eps[i]))
	}
	return bs
}

// Forward implements the interface Forwarder.
func (f *Forwarder) Forward(ctx *apigw.Context) (err error) {
	c := context.Background()
	if f.Timeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, f.Timeout)
		defer cancel()
	}

	var req Request
	if f.NewRequest == nil {
		req = simpleRequest{ctx}
	} else {
		req = f.NewRequest(ctx)
	}

	if _, err = f.RoundTrip(c, req); err == loadbalancer.ErrNoAvailableEndpoint {
		err = ship.ErrBadGateway.Newf("no available backends")
	}

	return
}
