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
	"errors"
	"sync"

	"github.com/xgfone/go-service/loadbalancer"
)

// NewBackendWithHealthCheck returns a new backend with the new HealthCheck.
func NewBackendWithHealthCheck(b Backend, hc HealthCheck) Backend {
	return hcBackend{Backend: b, hc: hc}
}

type hcBackend struct {
	hc HealthCheck
	Backend
}

func (b hcBackend) Unwrap() loadbalancer.Endpoint { return b.Backend }
func (b hcBackend) HealthCheck() HealthCheck      { return b.hc }

//////////////////////////////////////////////////////////////////////////////

type endpointBackend struct {
	Backend
	online bool
}

func newEndpointBackend(b Backend, online bool) endpointBackend {
	return endpointBackend{Backend: b, online: online}
}

func (eb endpointBackend) Unwrap() loadbalancer.Endpoint  { return eb.Backend }
func (eb endpointBackend) IsHealthy(context.Context) bool { return eb.online }
func (eb endpointBackend) MetaData() map[string]interface{} {
	md := eb.Backend.MetaData()
	md["online"] = eb.online
	return md
}

/////////////////////////////////////////////////////////////////////////////

// GroupBackendConfig is used to configure the group backend.
type GroupBackendConfig struct {
	UserData    interface{}
	HealthCheck HealthCheck
	IsHealthy   func(Backend) bool
}

// GroupBackend is the group backend, which implements the interface Backend
// and BackendGroup.
type GroupBackend struct {
	name string
	conf GroupBackendConfig

	lock     sync.RWMutex
	backends map[string]Backend
	updaters map[string]BackendGroupUpdater
}

var _ Backend = &GroupBackend{}
var _ BackendGroup = &GroupBackend{}

// NewGroupBackend returns a new group backend.
//
// If name is empty, it will panic.
func NewGroupBackend(name string, config *GroupBackendConfig) *GroupBackend {
	if name == "" {
		panic(errors.New("the group name must not be empty"))
	}

	var conf GroupBackendConfig
	if config != nil {
		conf = *config
	}

	return &GroupBackend{
		name:     name,
		conf:     conf,
		backends: make(map[string]Backend),
		updaters: make(map[string]BackendGroupUpdater),
	}
}

// Type implements the interface Backend.
func (b *GroupBackend) Type() string { return "group" }

// String implements the interface fmt.Stringer.
func (b *GroupBackend) String() string { return b.name }

// IsHealthy implements the interface Backend.
func (b *GroupBackend) IsHealthy(c context.Context) bool { return true }

// HealthCheck implements the interface Backend.
func (b *GroupBackend) HealthCheck() HealthCheck { return b.conf.HealthCheck }

// UserData implements the interface Backend.
func (b *GroupBackend) UserData() interface{} { return b.conf.UserData }

// MetaData implements the interface Backend.
func (b *GroupBackend) MetaData() map[string]interface{} {
	return map[string]interface{}{"name": b.name}
}

// RoundTrip implements the interface Backend.
func (b *GroupBackend) RoundTrip(c context.Context, r loadbalancer.Request) (
	loadbalancer.Response, error) {
	panic("GroupBackend.RoundTrip: not implemented")
}

// Close implements the interface io.Closer.
func (b *GroupBackend) Close() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, updater := range b.updaters {
		for _, backend := range b.backends {
			updater.DelBackendFromGroup(backend)
		}
		updater.DelBackendFromGroup(b)
	}

	b.backends = nil
	b.updaters = nil
	return nil
}

// Name implements the interface BackendGroup.
func (b *GroupBackend) Name() string { return b.name }

// AddBackend implements the interface BackendGroup.
func (b *GroupBackend) AddBackend(backend Backend) {
	addr := backend.String()
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.backends[addr]; !ok {
		b.backends[addr] = backend
		for _, u := range b.updaters {
			u.AddBackendFromGroup(backend)
		}
	}
}

// DelBackend implements the interface BackendGroup.
func (b *GroupBackend) DelBackend(backend Backend) {
	addr := backend.String()
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.backends[addr]; ok {
		delete(b.backends, addr)
		for _, u := range b.updaters {
			u.DelBackendFromGroup(backend)
		}
	}
}

// GetBackends implements the interface BackendGroup.
//
// If the group is not used by any forwarders, the online status of the backend
// is always true.
func (b *GroupBackend) GetBackends() []Backend {
	online := func(b Backend) bool { return true }

	b.lock.RLock()
	if b.conf.IsHealthy != nil && len(b.updaters) != 0 {
		online = func(backend Backend) bool { return b.conf.IsHealthy(backend) }
	}

	bs := make([]Backend, 0, len(b.backends))
	for _, backend := range b.backends {
		bs = append(bs, newEndpointBackend(backend, online(backend)))
	}
	b.lock.RUnlock()
	return bs
}

// GetUpdater implements the interface BackendGroup.
func (b *GroupBackend) GetUpdater(name string) BackendGroupUpdater {
	b.lock.RLock()
	u := b.updaters[name]
	b.lock.RUnlock()
	return u
}

// GetUpdaters implements the interface BackendGroup.
func (b *GroupBackend) GetUpdaters() []BackendGroupUpdater {
	b.lock.RLock()
	us := make([]BackendGroupUpdater, 0, len(b.updaters))
	for _, u := range b.updaters {
		us = append(us, u)
	}
	b.lock.RUnlock()
	return us
}

// AddUpdater implements the interface BackendGroup.
func (b *GroupBackend) AddUpdater(u BackendGroupUpdater) {
	name := u.Name()
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.updaters[name]; !ok {
		b.updaters[name] = u
		for _, backend := range b.backends {
			u.AddBackendFromGroup(backend)
		}
	}
}

// DelUpdater implements the interface BackendGroup.
func (b *GroupBackend) DelUpdater(u BackendGroupUpdater) {
	name := u.Name()
	b.lock.Lock()
	defer b.lock.Unlock()

	if u, ok := b.updaters[name]; ok {
		delete(b.updaters, name)
		for _, backend := range b.backends {
			u.DelBackendFromGroup(backend)
		}
	}
}
