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

// BackendGroupManager is used to manage the backend group.
type BackendGroupManager struct {
	name   string
	lock   sync.RWMutex
	groups map[string]BackendGroup
}

// NewBackendGroupManager returns a new backend group manager.
func NewBackendGroupManager(name string) *BackendGroupManager {
	return &BackendGroupManager{
		name:   name,
		groups: make(map[string]BackendGroup, 4),
	}
}

// Name returns the name of the backend group manager.
func (m *BackendGroupManager) Name() string { return m.name }

// AddOrNewBackendGroup is equal to m.AddBackendGroup(NewGroupBackend(name, config)).
func (m *BackendGroupManager) AddOrNewBackendGroup(name string, config *GroupBackendConfig) (bg BackendGroup) {
	m.lock.Lock()
	if g, ok := m.groups[name]; ok {
		bg = g
	} else {
		bg = NewGroupBackend(name, config)
		m.groups[name] = bg
	}
	m.lock.Unlock()
	return bg
}

// AddBackendGroup adds the backend group. If the backend group has been added,
// do nothing and return the added backend group.
func (m *BackendGroupManager) AddBackendGroup(bg BackendGroup) BackendGroup {
	m.lock.Lock()
	if g, ok := m.groups[bg.Name()]; ok {
		bg = g
	} else {
		m.groups[bg.Name()] = bg
	}
	m.lock.Unlock()
	return bg
}

// DelBackendGroupByName deletes the backend group by the name,
// and returns the deleted backend group.
//
// If the backend group does not exist, return nil.
func (m *BackendGroupManager) DelBackendGroupByName(name string) BackendGroup {
	m.lock.Lock()
	defer m.lock.Unlock()
	g, ok := m.groups[name]
	if ok {
		delete(m.groups, name)
		g.Close()
	}
	return g
}

// GetBackendGroup returns the backend group by the name.
//
// If the backend group does not exist, return nil.
func (m *BackendGroupManager) GetBackendGroup(name string) BackendGroup {
	m.lock.RLock()
	g := m.groups[name]
	m.lock.RUnlock()
	return g
}

// GetBackendGroups returns all the backend groups.
func (m *BackendGroupManager) GetBackendGroups() (bgs []BackendGroup) {
	m.lock.RLock()
	bgs = make([]BackendGroup, 0, len(m.groups))
	for _, g := range m.groups {
		bgs = append(bgs, g)
	}
	m.lock.RUnlock()
	return
}

// GetBackendGroupsByUpdaterName returns the list of the backend groups
// which add the updater.
func (m *BackendGroupManager) GetBackendGroupsByUpdaterName(name string) (bgs []BackendGroup) {
	m.lock.RLock()
	for _, g := range m.groups {
		if g.GetUpdater(name) != nil {
			bgs = append(bgs, g)
		}
	}
	m.lock.RUnlock()
	return
}

// DelUpdater deletes the backend group updater from all the backend group.
func (m *BackendGroupManager) DelUpdater(u BackendGroupUpdater) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, g := range m.groups {
		g.DelUpdater(u)
	}
}

// Close implements the interface io.Closer and releases all the resources.
func (m *BackendGroupManager) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, g := range m.groups {
		g.Close()
	}
	m.groups = nil
	return nil
}
