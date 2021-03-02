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

var bgms sync.Map

// RegisterBackendGroupManager registers and returns the backend group manager.
//
// If the backend group manager has been registered, do nothing
// and return the registered backend group manager.
func RegisterBackendGroupManager(m *BackendGroupManager) *BackendGroupManager {
	actual, _ := bgms.LoadOrStore(m.Name(), m)
	return actual.(*BackendGroupManager)
}

// UnregisterBackendGroupManager unregisters the backend group by the name,
// and returns the registered backend group.
//
// If the backend group does not exist, return nil.
func UnregisterBackendGroupManager(name string) *BackendGroupManager {
	if value, loaded := bgms.LoadAndDelete(name); loaded {
		return value.(*BackendGroupManager)
	}
	return nil
}

// GetBackendGroupManager returns the backend group manager by the name.
//
// If the backend group manager does not exist, return nil.
func GetBackendGroupManager(name string) (m *BackendGroupManager) {
	bgms.Range(func(key, value interface{}) bool {
		if key.(string) == name {
			m = value.(*BackendGroupManager)
			return false
		}
		return true
	})
	return
}

// GetBackendGroupManagers returns all the backend group manager.
func GetBackendGroupManagers() (ms []*BackendGroupManager) {
	bgms.Range(func(key, value interface{}) bool {
		ms = append(ms, value.(*BackendGroupManager))
		return true
	})
	return
}

// BackendGroupManager is used to manage the backend group.
type BackendGroupManager struct {
	name   string
	groups sync.Map
}

// NewBackendGroupManager returns a new backend group manager.
func NewBackendGroupManager(name string) *BackendGroupManager {
	return &BackendGroupManager{name: name}
}

// Name returns the name of the backend group manager.
func (m *BackendGroupManager) Name() string { return m.name }

// Add adds the backend group. If the backend group has been added,
// do nothing and return the added backend group.
func (m *BackendGroupManager) Add(bg *BackendGroup) *BackendGroup {
	actual, _ := m.groups.LoadOrStore(bg.Name(), bg)
	return actual.(*BackendGroup)
}

// Delete deletes the backend group by the name, and returns the added
// backend group. If the backend group does not exist, return nil.
func (m *BackendGroupManager) Delete(backendGroupName string) *BackendGroup {
	if value, loaded := m.groups.LoadAndDelete(backendGroupName); loaded {
		return value.(*BackendGroup)
	}
	return nil
}

// BackendGroup returns the backend group by the name.
//
// If the backend group does not exist, return nil.
func (m *BackendGroupManager) BackendGroup(name string) *BackendGroup {
	if value, ok := m.groups.Load(name); ok {
		return value.(*BackendGroup)
	}
	return nil
}

// BackendGroups returns all the backend groups.
func (m *BackendGroupManager) BackendGroups() (bgs []*BackendGroup) {
	m.groups.Range(func(key, value interface{}) bool {
		bgs = append(bgs, value.(*BackendGroup))
		return true
	})
	return
}

// BackendGroupsByUpdaterName returns the list of the backend groups
// which add the updater.
func (m *BackendGroupManager) BackendGroupsByUpdaterName(name string) (bgs []*BackendGroup) {
	m.groups.Range(func(key, value interface{}) bool {
		if bg := value.(*BackendGroup); bg.GetUpdater(name) != nil {
			bgs = append(bgs, bg)
		}
		return true
	})
	return
}

// DelUpdater deletes the backend updater from all the backend group.
func (m *BackendGroupManager) DelUpdater(u BackendUpdater) {
	m.groups.Range(func(key, value interface{}) bool {
		value.(*BackendGroup).DelUpdater(u)
		return true
	})
}

// Close releases all the resources.
func (m *BackendGroupManager) Close() {
	m.groups.Range(func(key, value interface{}) bool {
		value.(*BackendGroup).Close()
		m.groups.Delete(key)
		return true
	})
}
