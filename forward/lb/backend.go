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
	"io"
	"sync"

	lb "github.com/xgfone/go-loadbalancer"
)

// BackendCheckerDurationZero is ZERO of BackendCheckerDuration.
var BackendCheckerDurationZero BackendCheckerDuration

// Predefine some types about backend.
type (
	Request                = lb.Request
	Backend                = lb.Endpoint
	BackendState           = lb.EndpointState
	BackendChecker         = lb.EndpointChecker
	BackendCheckerFunc     = lb.EndpointCheckerFunc
	BackendCheckerDuration = lb.EndpointCheckerDuration
	ConnectionState        = lb.ConnectionState
)

// BackendInfo is the build information.
type BackendInfo struct {
	Online bool
	Backend
	BackendChecker
	BackendCheckerDuration
}

// BackendUpdater is used to add or delete the backend.
type BackendUpdater interface {
	// Name returns the name of the updater.
	Name() string

	// AddBackend adds the backend.
	//
	// If the backend has been added, do nothing.
	AddBackend(backend Backend)

	// DelBackendByID deletes the backend by the backend id.
	DelBackendByID(backendID string)
}

// BackendGroup is used to manage a group of Backend.
type BackendGroup interface {
	io.Closer

	// Name returns the name of the group.
	Name() string

	// AddBackendWithChecker adds the backend into the group.
	//
	// If the backend has been added, do nothing.
	AddBackendWithChecker(Backend, BackendChecker, BackendCheckerDuration)

	// DelBackendByID deletes the backend by the backend id from the group.
	DelBackendByID(backendID string)

	// GetBackends returns all the backends in the group.
	GetBackends() []BackendInfo

	// GetUpdaters returns all the backend updaters.
	GetUpdaters() []BackendUpdater

	// GetUpdaterByName returns the backend updater by the name.
	//
	// If the updater does not exist, return nil.
	GetUpdaterByName(name string) BackendUpdater

	// DelUpdaterByName deletes the backend updater by the name,
	// which should delete all the backends in the group from the updater.
	//
	// If the updater does not exist, do nothing.
	DelUpdaterByName(name string)

	// AddUpdater adds the backend updater, which should add all the backends
	// into the updater.
	//
	// If the updater has been added, do nothing.
	AddUpdater(BackendUpdater)
}

type groupEndpointUpdater struct{ *GroupBackend }

func (u groupEndpointUpdater) AddEndpoint(ep lb.Endpoint) { u.addEndpoint(ep) }
func (u groupEndpointUpdater) DelEndpoint(ep lb.Endpoint) { u.delEndpoint(ep.ID()) }
func (u groupEndpointUpdater) DelEndpointByID(id string)  { u.delEndpoint(id) }

// GroupBackend is the group backend, which implements the interface Backend
// and BackendGroup.
type GroupBackend struct {
	checker *lb.HealthCheck

	name     string
	lock     sync.RWMutex
	backends map[string]BackendInfo
	updaters map[string]BackendUpdater
}

var _ Backend = &GroupBackend{}
var _ BackendGroup = &GroupBackend{}

// NewGroupBackend returns a new group backend.
//
// If name is empty, it will panic.
func NewGroupBackend(name string) *GroupBackend {
	if name == "" {
		panic(errors.New("the group name must not be empty"))
	}

	b := &GroupBackend{
		name:     name,
		checker:  lb.NewHealthCheck(),
		backends: make(map[string]BackendInfo),
		updaters: make(map[string]BackendUpdater),
	}
	b.checker.AddUpdater(b.name, groupEndpointUpdater{b})

	return b
}

// ID implements the interface Backend.
func (b *GroupBackend) ID() string { return b.name }

// Type implements the interface Backend.
func (b *GroupBackend) Type() string { return "group" }

// State implements the interface Backend.
func (b *GroupBackend) State() (s BackendState) { return }

// MetaData implements the interface Backend.
func (b *GroupBackend) MetaData() map[string]interface{} {
	return map[string]interface{}{"name": b.name}
}

// RoundTrip implements the interface Backend.
func (b *GroupBackend) RoundTrip(context.Context, Request) (interface{}, error) {
	panic("GroupBackend.RoundTrip: not implemented")
}

// Close implements the interface BackendGroup.
func (b *GroupBackend) Close() error {
	b.lock.Lock()
	backends, updaters := b.backends, b.updaters
	b.backends, b.updaters = nil, nil
	b.lock.Unlock()
	b.checker.Stop()

	for _, updater := range updaters {
		updater.DelBackendByID(b.ID())
		for _, backend := range backends {
			updater.DelBackendByID(backend.Backend.ID())
		}
	}

	return nil
}

// Name implements the interface BackendGroup.
func (b *GroupBackend) Name() string { return b.name }

// AddBackendWithChecker implements the interface BackendGroup.
func (b *GroupBackend) AddBackendWithChecker(backend Backend,
	checker BackendChecker, duration BackendCheckerDuration) {
	id := backend.ID()
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.backends[id]; !ok {
		online := true
		if checker == nil {
			for _, u := range b.updaters {
				u.AddBackend(backend)
			}
		} else {
			online = false
			b.checker.AddEndpoint(backend, checker, duration)
		}

		b.backends[id] = BackendInfo{
			Online:                 online,
			Backend:                backend,
			BackendChecker:         checker,
			BackendCheckerDuration: duration,
		}
	}
}

// DelBackendByID implements the interface BackendGroup.
func (b *GroupBackend) DelBackendByID(backendID string) {
	if b.delBackendByID(backendID) {
		b.checker.DelEndpointByID(backendID)
	}
}

func (b *GroupBackend) delBackendByID(backendID string) (checker bool) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if backend, ok := b.backends[backendID]; ok {
		checker = backend.BackendChecker != nil
		delete(b.backends, backendID)
		for _, u := range b.updaters {
			u.DelBackendByID(backendID)
		}
	}
	return
}

// GetBackends implements the interface BackendGroup.
func (b *GroupBackend) GetBackends() []BackendInfo {
	b.lock.RLock()
	bs := make([]BackendInfo, 0, len(b.backends))
	for _, backend := range b.backends {
		if !backend.Online {
			backend.Online = b.checker.IsHealthy(backend.Backend.ID())
		}
		bs = append(bs, backend)
	}
	b.lock.RUnlock()
	return bs
}

// GetUpdaters implements the interface BackendGroup.
func (b *GroupBackend) GetUpdaters() []BackendUpdater {
	b.lock.RLock()
	us := make([]BackendUpdater, 0, len(b.updaters))
	for _, u := range b.updaters {
		us = append(us, u)
	}
	b.lock.RUnlock()
	return us
}

// GetUpdaterByName implements the interface BackendGroup.
func (b *GroupBackend) GetUpdaterByName(name string) BackendUpdater {
	b.lock.RLock()
	u := b.updaters[name]
	b.lock.RUnlock()
	return u
}

// AddUpdater implements the interface BackendGroup.
func (b *GroupBackend) AddUpdater(u BackendUpdater) {
	name := u.Name()
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, ok := b.updaters[name]; !ok {
		b.updaters[name] = u
		for _, backend := range b.backends {
			if backend.Online {
				u.AddBackend(backend.Backend)
			} else {
				for _, ep := range b.checker.GetEndpoints() {
					if ep.Healthy {
						u.AddBackend(backend.Backend)
					}
				}
			}
		}
	}
}

// DelUpdaterByName implements the interface BackendGroup.
func (b *GroupBackend) DelUpdaterByName(name string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if u, ok := b.updaters[name]; ok {
		delete(b.updaters, name)
		for _, backend := range b.backends {
			u.DelBackendByID(backend.Backend.ID())
		}
	}
}

func (b *GroupBackend) addEndpoint(ep lb.Endpoint) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, u := range b.updaters {
		u.AddBackend(ep)
	}
}

func (b *GroupBackend) delEndpoint(id string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	for _, u := range b.updaters {
		u.DelBackendByID(id)
	}
}
