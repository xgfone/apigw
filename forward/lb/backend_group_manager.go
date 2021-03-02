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

var (
	bgms     = make(map[string]*BackendGroupManager)
	bgmslock sync.RWMutex
)

// RegisterBackendGroupManager registers and returns the backend group manager.
//
// If the backend group manager has been registered, do nothing
// and return the registered backend group manager.
func RegisterBackendGroupManager(m *BackendGroupManager) *BackendGroupManager {
	bgmslock.Lock()
	if bgm, ok := bgms[m.name]; ok {
		m = bgm
	} else {
		bgms[m.name] = m
	}
	bgmslock.Unlock()
	return m
}

// UnregisterBackendGroupManager unregisters the backend group by the name,
// and returns the registered backend group.
//
// If the backend group does not exist, return nil.
func UnregisterBackendGroupManager(name string) *BackendGroupManager {
	bgmslock.Lock()
	m, ok := bgms[name]
	if ok {
		delete(bgms, name)
	}
	bgmslock.Unlock()
	return m
}

// GetBackendGroupManager returns the backend group manager by the name.
//
// If the backend group manager does not exist, return nil.
func GetBackendGroupManager(name string) (m *BackendGroupManager) {
	bgmslock.RLock()
	m = bgms[name]
	bgmslock.RUnlock()
	return
}

// GetBackendGroupManagers returns all the backend group manager.
func GetBackendGroupManagers() (ms []*BackendGroupManager) {
	bgmslock.RLock()
	ms = make([]*BackendGroupManager, 0, len(bgms))
	for _, m := range bgms {
		ms = append(ms, m)
	}
	bgmslock.RUnlock()
	return
}

// BackendGroupManager is used to manage the backend group.
type BackendGroupManager struct {
	name   string
	lock   sync.RWMutex
	groups map[string]*BackendGroup
}

// NewBackendGroupManager returns a new backend group manager.
func NewBackendGroupManager(name string) *BackendGroupManager {
	return &BackendGroupManager{
		name:   name,
		groups: make(map[string]*BackendGroup, 4),
	}
}

// Name returns the name of the backend group manager.
func (m *BackendGroupManager) Name() string { return m.name }

// Add adds the backend group. If the backend group has been added,
// do nothing and return the added backend group.
func (m *BackendGroupManager) Add(bg *BackendGroup) *BackendGroup {
	m.lock.Lock()
	if g, ok := m.groups[bg.name]; ok {
		bg = g
	} else {
		m.groups[bg.name] = bg
	}
	m.lock.Unlock()
	return bg
}

// Delete deletes the backend group by the name, and returns the added
// backend group. If the backend group does not exist, return nil.
func (m *BackendGroupManager) Delete(backendGroupName string) *BackendGroup {
	m.lock.Lock()
	g, ok := m.groups[backendGroupName]
	if ok {
		delete(m.groups, backendGroupName)
	}
	m.lock.Unlock()
	return g
}

// BackendGroup returns the backend group by the name.
//
// If the backend group does not exist, return nil.
func (m *BackendGroupManager) BackendGroup(name string) *BackendGroup {
	m.lock.RLock()
	g := m.groups[name]
	m.lock.RUnlock()
	return g
}

// BackendGroups returns all the backend groups.
func (m *BackendGroupManager) BackendGroups() (bgs []*BackendGroup) {
	m.lock.RLock()
	bgs = make([]*BackendGroup, 0, len(m.groups))
	for _, g := range m.groups {
		bgs = append(bgs, g)
	}
	m.lock.RUnlock()
	return
}

// BackendGroupsByUpdaterName returns the list of the backend groups
// which add the updater.
func (m *BackendGroupManager) BackendGroupsByUpdaterName(name string) (bgs []*BackendGroup) {
	m.lock.RLock()
	for _, g := range m.groups {
		if g.GetUpdater(name) != nil {
			bgs = append(bgs, g)
		}
	}
	m.lock.RUnlock()
	return
}

// DelUpdater deletes the backend updater from all the backend group.
func (m *BackendGroupManager) DelUpdater(u BackendUpdater) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, g := range m.groups {
		g.DelUpdater(u)
	}
}

// Close releases all the resources.
func (m *BackendGroupManager) Close() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, g := range m.groups {
		g.Close()
	}
	m.groups = nil
}
