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

import "sync"

// BackendUpdater is used to update the backend.
type BackendUpdater interface {
	Name() string
	AddBackend(b Backend)
	DelBackend(b Backend)
}

// BackendGroup is used to manage the group of Backends.
type BackendGroup struct {
	name     string
	lock     sync.RWMutex
	backends map[string]Backend
	updaters map[string]BackendUpdater
}

// NewBackendGroup returns a new BackendGroup with the name.
func NewBackendGroup(name string) *BackendGroup {
	return &BackendGroup{
		name:     name,
		backends: make(map[string]Backend),
		updaters: make(map[string]BackendUpdater),
	}
}

// Name returns the name of the backend group.
func (bg *BackendGroup) Name() string { return bg.name }

// AddBackend adds the backend into the group.
func (bg *BackendGroup) AddBackend(b Backend) {
	addr := b.String()
	bg.lock.Lock()
	defer bg.lock.Unlock()
	if _, ok := bg.backends[addr]; !ok {
		bg.backends[addr] = b

		for _, u := range bg.updaters {
			u.AddBackend(b)
		}
	}
}

// DelBackend deletes the backend from the group.
func (bg *BackendGroup) DelBackend(b Backend) {
	addr := b.String()
	bg.lock.Lock()
	defer bg.lock.Unlock()
	if _, ok := bg.backends[addr]; ok {
		delete(bg.backends, addr)

		for _, u := range bg.updaters {
			u.DelBackend(b)
		}
	}
}

// AddBackends adds a set of Backends.
func (bg *BackendGroup) AddBackends(backends []Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		bg.AddBackend(backends[i])
	}
}

// DelBackends deletes a set of Backends.
func (bg *BackendGroup) DelBackends(backends []Backend) {
	for i, _len := 0, len(backends); i < _len; i++ {
		bg.DelBackend(backends[i])
	}
}

// Backends returns the list of the backends in the current backend group.
func (bg *BackendGroup) Backends() (bs []Backend) {
	bg.lock.RLock()
	bs = make([]Backend, 0, len(bg.backends))
	for _, backend := range bg.backends {
		bs = append(bs, backend)
	}
	bg.lock.RUnlock()
	return
}

// GetUpdater returns the backend updater by the name.
//
// If the backend updater does not exist, return nil.
func (bg *BackendGroup) GetUpdater(name string) (u BackendUpdater) {
	bg.lock.RLock()
	u = bg.updaters[name]
	bg.lock.RUnlock()
	return
}

// GetUpdaters returns all the backend updaters.
func (bg *BackendGroup) GetUpdaters() (us []BackendUpdater) {
	bg.lock.RLock()
	us = make([]BackendUpdater, 0, len(bg.updaters))
	for _, u := range bg.updaters {
		us = append(us, u)
	}
	bg.lock.RUnlock()
	return
}

// AddUpdater adds the updater, which will be called when adding or deleting
// a backend.
func (bg *BackendGroup) AddUpdater(u BackendUpdater) {
	name := u.Name()
	bg.lock.Lock()
	defer bg.lock.Unlock()

	if _, ok := bg.updaters[name]; !ok {
		bg.updaters[name] = u
		for _, b := range bg.backends {
			u.AddBackend(b)
		}
	}
}

// DelUpdater is equal to DelUpdaterByName(u.Name()).
func (bg *BackendGroup) DelUpdater(u BackendUpdater) {
	bg.DelUpdaterByName(u.Name())
}

// DelUpdaterByName deletes the updater, which will be called when adding
// or deleting a backend.
func (bg *BackendGroup) DelUpdaterByName(updaterName string) {
	bg.lock.Lock()
	defer bg.lock.Unlock()
	if u, ok := bg.updaters[updaterName]; ok {
		delete(bg.updaters, updaterName)
		for _, b := range bg.backends {
			u.DelBackend(b)
		}
	}
}

// Close releases all the resources.
func (bg *BackendGroup) Close() {
	bg.lock.Lock()
	defer bg.lock.Unlock()

	for _, backend := range bg.backends {
		for _, updater := range bg.updaters {
			updater.DelBackend(backend)
		}
	}

	bg.backends = nil
	bg.updaters = nil
}
