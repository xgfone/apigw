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
	"sync/atomic"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/go-loadbalancer"
	"github.com/xgfone/ship/v4"
)

// ErrNoAvailableBackends represents that there is no available backends
// to forward the request.
var ErrNoAvailableBackends = errors.New("no available backends")

// Predefine some type aliases.
type (
	Selector = loadbalancer.Selector
	Session  = loadbalancer.Session
)

// Forwarder is a route forwarder to forward the request from the route.
type Forwarder struct {
	*loadbalancer.LoadBalancer
	NewRequest func(c *apigw.Context) HTTPRequest // Default: NewRequestWithXSessionId(c)

	lock     sync.RWMutex
	backends map[string]BackendInfo

	timeout int64
	checker *loadbalancer.HealthCheck
}

// NewForwarder returns a new backend Forwarder, which will pass HTTRequest
// as Request to call the method RoundTrip of Backend.
//
// The forwarder has also implemented the interface BackendGroupUpdater
// and loadbalancer.ProviderEndpointManager.
func NewForwarder(name string) *Forwarder {
	lb := loadbalancer.NewLoadBalancer(name, nil)
	lb.Session = loadbalancer.NewMemorySession(time.Minute)
	lb.FailRetry = loadbalancer.FailTry(1, time.Millisecond*10)

	checker := loadbalancer.NewHealthCheck()
	checker.AddUpdater(lb.Name(), lb)

	return &Forwarder{
		LoadBalancer: lb,
		checker:      checker,
		backends:     make(map[string]BackendInfo),
	}
}

// Close implements the interface io.Closer.
func (f *Forwarder) Close() error {
	f.lock.Lock()
	backends := f.backends
	f.backends = nil
	f.lock.Unlock()

	for _, backend := range backends {
		if group, ok := backend.Backend.(BackendGroup); ok {
			group.DelUpdaterByName(f.Name())
		}
	}

	f.checker.Stop()
	f.LoadBalancer.Session.Close()
	return f.LoadBalancer.Close()
}

// Forward implements the interface apigw.Forwarder.
func (f *Forwarder) Forward(ctx *apigw.Context) (err error) {
	c := context.Background()
	if timeout := f.GetMaxTimeout(); timeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, timeout)
		defer cancel()
	}

	var req Request
	if f.NewRequest == nil {
		req = simpleRequest{ctx}
	} else {
		req = f.NewRequest(ctx)
	}

	switch _, err = f.RoundTrip(c, req); err {
	case loadbalancer.ErrNoAvailableEndpoints, ErrNoAvailableBackends:
		err = ship.ErrBadGateway.New(ErrNoAvailableBackends)
	}
	return
}

// SetSessionTimeout resets the sesstion timeout.
func (f *Forwarder) SetSessionTimeout(timeout time.Duration) {
	f.LoadBalancer.SetSessionTimeout(timeout)
}

// GetSessionTimeout returns the sesstion timeout.
func (f *Forwarder) GetSessionTimeout() time.Duration {
	return f.LoadBalancer.GetSessionTimeout()
}

// SetSelectorByName is equal to SetSelector, but get the selector
// from the global registered selectors as the new session.
func (f *Forwarder) SetSelectorByName(selectorName string) (err error) {
	if s := loadbalancer.GetSelector(selectorName); s != nil {
		f.SetSelector(s)
		return nil
	}
	return fmt.Errorf("no forwarding policy selector named '%s'", selectorName)
}

// SetSelector sets the selector to new.
func (f *Forwarder) SetSelector(new Selector) {
	f.Provider.(loadbalancer.SelectorGetSetter).SetSelector(new)
}

// GetSelector returns the forwarding policy selector.
func (f *Forwarder) GetSelector() Selector {
	return f.Provider.(loadbalancer.SelectorGetSetter).GetSelector()
}

// SetMaxTimeout sets the maximum timeout to forward the request.
func (f *Forwarder) SetMaxTimeout(timeout time.Duration) {
	atomic.StoreInt64(&f.timeout, int64(timeout))
}

// GetMaxTimeout returns the maximum timeout to forward the request.
func (f *Forwarder) GetMaxTimeout() (timeout time.Duration) {
	return time.Duration(atomic.LoadInt64(&f.timeout))
}

// GetBackends returns all the backends.
func (f *Forwarder) GetBackends() []BackendInfo {
	f.lock.RLock()
	bs := make([]BackendInfo, 0, len(f.backends))
	for _, backend := range f.backends {
		if !backend.Online {
			backend.Online = f.checker.IsHealthy(backend.Backend.ID())
		}
		bs = append(bs, backend)
	}
	f.lock.RUnlock()
	return bs
}

// AddBackendGroup adds the group as the backend, which is equal to
//   f.AddBackendWithChecker(group.(Backend), group, nil, BackendCheckerDurationZero)
func (f *Forwarder) AddBackendGroup(group BackendGroup) {
	if f.addBackend(group.(Backend), group, nil, BackendCheckerDurationZero) {
		group.AddUpdater(forwardUpdater{f})
	}
}

// AddBackendWithChecker adds the backend with the checker.
//
// If the backend has been added, do nothing.
func (f *Forwarder) AddBackendWithChecker(backend Backend,
	checker BackendChecker, duration BackendCheckerDuration) {

	if group, ok := backend.(BackendGroup); !ok {
		f.addBackend(backend, nil, checker, duration)
	} else if f.addBackend(backend, group, checker, BackendCheckerDuration{}) {
		group.AddUpdater(forwardUpdater{f})
	}
}

// DelBackendByID deletes the backend by the backend id.
func (f *Forwarder) DelBackendByID(backendID string) {
	f.lock.Lock()
	backend, ok := f.backends[backendID]
	if ok {
		delete(f.backends, backendID)
	}
	f.lock.Unlock()

	if ok {
		if group, ok := backend.Backend.(BackendGroup); ok {
			group.DelUpdaterByName(f.Name())
		} else {
			f.delEndpointByID(backendID)
		}
	}
}

func (f *Forwarder) addBackend(backend Backend, group BackendGroup,
	checker BackendChecker, duration BackendCheckerDuration) (ok bool) {
	id := backend.ID()
	f.lock.Lock()
	defer f.lock.Unlock()

	if _, ok = f.backends[id]; !ok {
		if group != nil {
			f.backends[id] = BackendInfo{
				Online:                 true,
				Backend:                backend,
				BackendChecker:         checker,
				BackendCheckerDuration: duration,
			}
		} else if checker == nil {
			f.addEndpoint(backend)
			f.backends[id] = BackendInfo{
				Online:                 true,
				Backend:                backend,
				BackendChecker:         checker,
				BackendCheckerDuration: duration,
			}
		} else {
			f.checker.AddEndpoint(backend, checker, duration)
			f.backends[id] = BackendInfo{
				Backend:                backend,
				BackendChecker:         checker,
				BackendCheckerDuration: duration,
			}
		}
	}

	return !ok
}

func (f *Forwarder) addEndpoint(backend Backend) {
	f.LoadBalancer.AddEndpoint(backend)
}

func (f *Forwarder) delEndpointByID(backendID string) {
	f.LoadBalancer.DelEndpointByID(backendID)
}

type forwardUpdater struct{ *Forwarder }

func (u forwardUpdater) AddBackend(backend Backend) { u.addEndpoint(backend) }
func (u forwardUpdater) DelBackendByID(id string)   { u.delEndpointByID(id) }
