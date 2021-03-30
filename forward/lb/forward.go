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

// Package lb implements a backend forwarder based on the loadbalancer.
package lb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v4"
)

// ErrNoAvailableBackends represents that there is no available backends
// to forward the request.
var ErrNoAvailableBackends = errors.New("no available backends")

// Predefine some type aliases.
type (
	HealthCheckOption = loadbalancer.HealthCheckOption
	ConnectionState   = loadbalancer.ConnectionState
	EndpointState     = loadbalancer.EndpointState
	BackendState      = loadbalancer.EndpointState
	Selector          = loadbalancer.Selector
	Session           = loadbalancer.Session
	Request           = loadbalancer.Request
)

var _ apigw.Forwarder = &Forwarder{}
var _ BackendUpdater = &Forwarder{}
var _ BackendGroupUpdater = &Forwarder{}
var _ loadbalancer.SessionManager = &Forwarder{}
var _ loadbalancer.SelectorManager = &Forwarder{}
var _ loadbalancer.EndpointUpdater = &Forwarder{}

// Forwarder is a route forwarder to forward the request from the route.
type Forwarder struct {
	*loadbalancer.LoadBalancer
	HealthCheck *loadbalancer.HealthCheck          // Default: nil
	NewRequest  func(c *apigw.Context) HTTPRequest // Default: NewRequestWithXSessionId(c)
	MaxTimeout  time.Duration                      // Default: 0

	lock     sync.RWMutex
	hcoption HealthCheckOption
	backends map[string]Backend
}

// NewForwarder returns a new backend Forwarder, which will pass HTTRequest
// as Request to call the method RoundTrip of Backend.
//
// The forwarder has also implemented the interface BackendGroupUpdater
// and loadbalancer.ProviderEndpointManager.
func NewForwarder(name string) *Forwarder {
	p := loadbalancer.NewSessionProvider(nil, nil, 0)
	lb := loadbalancer.NewLoadBalancer(name, p)
	return &Forwarder{LoadBalancer: lb, backends: make(map[string]Backend)}
}

// Close implements the interface io.Closer.
func (f *Forwarder) Close() error {
	f.lock.RLock()
	backends := make([]Backend, 0, len(f.backends))
	for _, backend := range f.backends {
		backends = append(backends, backend)
	}
	f.lock.RUnlock()
	f.DelBackends(backends...)

	if f.HealthCheck != nil {
		f.HealthCheck.DelEndpointsByUpdater(f)
		f.HealthCheck.UnsubscribeByUpdater(f)
	}
	return f.LoadBalancer.Close()
}

// Forward implements the interface apigw.Forwarder.
func (f *Forwarder) Forward(ctx *apigw.Context) (err error) {
	c := context.Background()
	if f.MaxTimeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, f.MaxTimeout)
		defer cancel()
	}

	var req Request
	if f.NewRequest == nil {
		req = simpleRequest{ctx}
	} else {
		req = f.NewRequest(ctx)
	}

	switch _, err = f.RoundTrip(c, req); err {
	case loadbalancer.ErrNoAvailableEndpoint, ErrNoAvailableBackends:
		err = ship.ErrBadGateway.New(ErrNoAvailableBackends)
	}
	return
}

// SetSession sets the session to new and returns the old if new and old
// are not the same session. Or do nothing and return nil.
func (f *Forwarder) SetSession(new Session) (old Session) {
	return f.Provider.(loadbalancer.SessionManager).SetSession(new)
}

// GetSession returns the session manager.
func (f *Forwarder) GetSession() Session {
	return f.Provider.(loadbalancer.SessionManager).GetSession()
}

// SetSessionTimeout resets the sesstion timeout.
func (f *Forwarder) SetSessionTimeout(timeout time.Duration) {
	f.Provider.(interface{ SetSessionTimeout(time.Duration) }).SetSessionTimeout(timeout)
}

// GetSessionTimeout returns the sesstion timeout.
func (f *Forwarder) GetSessionTimeout() time.Duration {
	return f.Provider.(interface{ GetSessionTimeout() time.Duration }).GetSessionTimeout()
}

// SetSelector is equal to SetSelector, but get the session from the global
// registered selectors as the new session.
func (f *Forwarder) SetSelectorByString(selector string) (old Selector, err error) {
	if s := loadbalancer.GetSelector(selector); s != nil {
		return f.SetSelector(s), nil
	}
	return nil, fmt.Errorf("no forwarding policy selector named '%s'", selector)
}

// SetSelector sets the selector to new and returns the old if new and old
// are not the same selector. Or do nothing and return nil.
func (f *Forwarder) SetSelector(new Selector) (old Selector) {
	return f.Provider.(loadbalancer.SelectorManager).SetSelector(new)
}

// GetSelector returns the forwarding policy selector.
func (f *Forwarder) GetSelector() Selector {
	return f.Provider.(loadbalancer.SelectorManager).GetSelector()
}

// SetHealthCheckOption sets the health check option.
func (f *Forwarder) SetHealthCheckOption(option HealthCheckOption) {
	f.lock.Lock()
	f.hcoption = option
	f.lock.Unlock()
}

// GetHealthCheckOption returns the health check option.
func (f *Forwarder) GetHealthCheckOption() HealthCheckOption {
	f.lock.RLock()
	option := f.hcoption
	f.lock.RUnlock()
	return option
}

// AddBackendFromGroup implements the interface BackendGroupUpdater.
func (f *Forwarder) AddBackendFromGroup(b Backend) {
	f.addBackend(b, f.GetHealthCheckOption())
}

// DelBackendFromGroup implements the interface BackendGroupUpdater.
func (f *Forwarder) DelBackendFromGroup(b Backend) {
	if _, ok := b.(BackendGroup); ok {
		addr := b.String()
		f.lock.Lock()
		delete(f.backends, addr)
		f.lock.Unlock()
		return
	}
	f.delBackend(b)
}

// AddBackends is a convenient function to add a set of backends.
func (f *Forwarder) AddBackends(backends ...Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.AddBackend(backends[i])
	}
}

// DelBackends is a convenient function to delete a set of backends.
func (f *Forwarder) DelBackends(backends ...Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.DelBackend(backends[i])
	}
}

// GetBackends returns all the backends in the forwarder.
func (f *Forwarder) GetBackends() []Backend {
	online := func(b Backend) bool { return true }
	if f.HealthCheck != nil {
		online = func(b Backend) bool {
			if _, ok := b.(BackendGroup); ok {
				return b.IsHealthy(context.Background())
			}
			return f.HealthCheck.IsHealthy(b.String())
		}
	}

	f.lock.RLock()
	bs := make([]Backend, 0, len(f.backends))
	for _, b := range f.backends {
		bs = append(bs, newEndpointBackend(b, online(b)))
	}
	f.lock.RUnlock()
	return bs
}

// AddBackend implements the interface BackendUpdater.
func (f *Forwarder) AddBackend(b Backend) {
	addr := b.String()
	f.lock.Lock()
	if _, ok := f.backends[addr]; ok {
		f.lock.Unlock()
		return
	}
	option := f.hcoption
	f.backends[addr] = b
	f.lock.Unlock()

	if gb, ok := b.(BackendGroup); ok {
		gb.AddUpdater(f)
	} else {
		f.addBackend(b, option)
	}
}

// DelBackend implements the interface BackendUpdater.
func (f *Forwarder) DelBackend(b Backend) {
	addr := b.String()
	f.lock.Lock()
	b, ok := f.backends[addr]
	if !ok {
		f.lock.Unlock()
		return
	}
	delete(f.backends, addr)
	f.lock.Unlock()

	if gb, ok := b.(BackendGroup); ok {
		gb.DelUpdater(f)
	} else {
		f.delBackend(b)
	}
}

// DelBackendByString is the same as DelBackend, but use the string backend
// instead.
func (f *Forwarder) DelBackendByString(backend string) {
	f.lock.Lock()
	b, ok := f.backends[backend]
	if !ok {
		f.lock.Unlock()
		return
	}
	delete(f.backends, backend)
	f.lock.Unlock()

	if gb, ok := b.(BackendGroup); ok {
		gb.DelUpdater(f)
	} else {
		f.delBackend(b)
	}
}

// DelBackendsByString is the same as DelBackends, but use the string backend
// instead.
func (f *Forwarder) DelBackendsByString(backends ...string) {
	for _, backend := range backends {
		f.DelBackendByString(backend)
	}
}

func (f *Forwarder) addBackend(b Backend, o HealthCheckOption) {
	if f.HealthCheck == nil {
		f.AddEndpoint(b)
		return
	}

	f.HealthCheck.Subscribe(b.String(), f)
	f.HealthCheck.AddEndpointWithDuration(b, o)
}

func (f *Forwarder) delBackend(b Backend) {
	if f.HealthCheck != nil {
		f.HealthCheck.Unsubscribe(b.String())
		f.HealthCheck.DelEndpoint(b)
	}
	f.DelEndpoint(b)
}
