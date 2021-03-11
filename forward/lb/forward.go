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
	"io"
	"sync"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// ErrNoAvailableBackends represents that there is no available backends
// to forward the request.
var ErrNoAvailableBackends = errors.New("no available backends")

// Request is the type alias of loadbalancer.Request, which can be used
// when you implement a backend.
type Request = loadbalancer.Request

// Response is the type alias of loadbalancer.Response, which can be used
// when you implement a backend.
type Response = loadbalancer.Response

// HealthCheck is the information of the health check of the endpoint.
type HealthCheck struct {
	Timeout  time.Duration `json:"timeout,omitempty" xml:"timeout,omitempty"`
	Interval time.Duration `json:"interval,omitempty" xml:"interval,omitempty"`
	RetryNum int           `json:"retrynum,omitempty" xml:"retrynum,omitempty"`
}

// Backend represents the forwarded backend used by Forwarder.
//
// Notice: The Forwarder backend must implement the interface.
type Backend interface {
	loadbalancer.Endpoint
	HealthCheck() HealthCheck
}

// BackendUnwrap is used to unwrap the inner Backend.
type BackendUnwrap interface {
	// Unwrap unwraps the inner Backend, but returns nil instead if no inner
	// Backend.
	UnwrapBackend() Backend
}

// BackendUpdater is used to add or delete the backend.
type BackendUpdater interface {
	io.Closer

	// Name returns the name of the group.
	Name() string

	// AddBackend adds the backend into the group.
	//
	// If the backend has been added, do nothing.
	AddBackend(Backend)

	// DelBackend deletes the backend from the group.
	//
	// If the backend does not exist, do nothing.
	DelBackend(Backend)
}

// Forwarder is the forwarder based on the loadbalancer.
type Forwarder interface {
	apigw.Forwarder
	BackendUpdater

	GetBackends() []Backend
	AddBackends(...Backend)
	DelBackends(...Backend)
}

// UnwrapBackend returns the most inner backend. Or return itself.
func UnwrapBackend(backend Backend) Backend {
	for {
		if bu, ok := backend.(BackendUnwrap); !ok {
			break
		} else if b := bu.UnwrapBackend(); b == nil {
			break
		} else {
			backend = b
		}
	}

	return backend
}

// ForwarderConfig is used to configure the LB forwarder.
type ForwarderConfig struct {
	MaxTimeout time.Duration
	NewRequest func(*apigw.Context) HTTPRequest

	HealthCheck        *loadbalancer.HealthCheck
	UpdateLoadBalancer func(*loadbalancer.LoadBalancer)
}

// NewForwarder returns a new backend Forwarder, which will pass HTTRequest
// as Request to call the method RoundTrip of Backend.
//
// The forwarder has also implemented the interface BackendGroupUpdater
// and loadbalancer.ProviderEndpointManager.
func NewForwarder(name string, config *ForwarderConfig) Forwarder {
	var conf ForwarderConfig
	if config != nil {
		conf = *config
	}

	lb := loadbalancer.NewLoadBalancer(nil)
	lb.Name = name
	if conf.UpdateLoadBalancer != nil {
		conf.UpdateLoadBalancer(lb)
	}

	return &forwarder{
		lb:         lb,
		hc:         conf.HealthCheck,
		timeout:    conf.MaxTimeout,
		newRequest: conf.NewRequest,
		backends:   make(map[string]Backend),
	}
}

var _ BackendGroupUpdater = &forwarder{}
var _ loadbalancer.ProviderEndpointManager = &forwarder{}

type forwarder struct {
	hc *loadbalancer.HealthCheck
	lb *loadbalancer.LoadBalancer

	timeout    time.Duration
	newRequest func(*apigw.Context) HTTPRequest

	lock     sync.RWMutex
	backends map[string]Backend
}

func (f *forwarder) Name() string { return f.lb.Name }

func (f *forwarder) Close() error {
	f.lock.RLock()
	backends := make([]Backend, 0, len(f.backends))
	for _, backend := range f.backends {
		backends = append(backends, backend)
	}
	f.lock.RUnlock()
	f.DelBackends(backends...)

	if f.hc != nil {
		f.hc.DelEndpointsByUpdater(f)
		f.hc.UnsubscribeByUpdater(f)
	}

	return f.lb.Close()
}

func (f *forwarder) Forward(ctx *apigw.Context) (err error) {
	c := context.Background()
	if f.timeout > 0 {
		var cancel func()
		c, cancel = context.WithTimeout(c, f.timeout)
		defer cancel()
	}

	var req Request
	if f.newRequest == nil {
		req = simpleRequest{ctx}
	} else {
		req = f.newRequest(ctx)
	}

	switch _, err = f.lb.RoundTrip(c, req); err {
	case loadbalancer.ErrNoAvailableEndpoint, ErrNoAvailableBackends:
		err = ship.ErrBadGateway.New(ErrNoAvailableBackends)
	}
	return
}

func (f *forwarder) Endpoints() loadbalancer.Endpoints {
	return f.lb.EndpointManager().Endpoints()
}

func (f *forwarder) AddEndpoint(ep loadbalancer.Endpoint) {
	f.lb.EndpointManager().AddEndpoint(ep)
}

func (f *forwarder) DelEndpoint(ep loadbalancer.Endpoint) {
	f.lb.EndpointManager().DelEndpoint(ep)
}

func (f *forwarder) AddBackendFromGroup(b Backend) { f.addBackend(b) }

func (f *forwarder) DelBackendFromGroup(b Backend) {
	if _, ok := b.(BackendGroup); ok {
		addr := b.String()
		f.lock.Lock()
		delete(f.backends, addr)
		f.lock.Unlock()
		return
	}

	f.delBackend(b)
}

func (f *forwarder) AddBackends(backends ...Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.AddBackend(backends[i])
	}
}

func (f *forwarder) DelBackends(backends ...Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		f.DelBackend(backends[i])
	}
}

func (f *forwarder) GetBackends() []Backend {
	online := func(b Backend) bool { return true }
	if f.hc != nil {
		online = func(b Backend) bool {
			if f.hc.IsHealthy(b.String()) {
				return true
			}

			if _, ok := UnwrapBackend(b).(BackendGroup); ok {
				return b.IsHealthy(context.Background())
			}

			return false
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

func (f *forwarder) AddBackend(b Backend) {
	addr := b.String()
	f.lock.Lock()
	if _, ok := f.backends[addr]; ok {
		f.lock.Unlock()
		return
	}
	f.backends[addr] = b
	f.lock.Unlock()

	if gb, ok := UnwrapBackend(b).(BackendGroup); ok {
		gb.AddUpdater(f)
	} else {
		f.addBackend(b)
	}
}

func (f *forwarder) DelBackend(b Backend) {
	addr := b.String()
	f.lock.Lock()
	b, ok := f.backends[addr]
	if !ok {
		f.lock.Unlock()
		return
	}
	delete(f.backends, addr)
	f.lock.Unlock()

	if gb, ok := UnwrapBackend(b).(BackendGroup); ok {
		gb.DelUpdater(f)
	} else {
		f.delBackend(b)
	}
}

func (f *forwarder) addBackend(b Backend) {
	if f.hc == nil {
		f.AddEndpoint(b)
		return
	}

	hc := b.HealthCheck()
	if hc.Interval == 0 {
		hc.Interval = f.hc.Interval
	}
	if hc.Timeout == 0 {
		hc.Timeout = f.hc.Timeout
	}
	if hc.RetryNum == 0 {
		hc.RetryNum = f.hc.RetryNum
	}

	f.hc.Subscribe(b.String(), f)
	f.hc.AddEndpointWithDuration(b, hc.Interval, hc.Timeout, hc.RetryNum)
}

func (f *forwarder) delBackend(b Backend) {
	if f.hc != nil {
		f.hc.Unsubscribe(b.String())
		f.hc.DelEndpoint(b)
	}
	f.DelEndpoint(b)
}
