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

package plugin

import (
	"fmt"
	"sync"
)

// Manager is used to manage a set of plugins.
type Manager struct {
	lock    sync.RWMutex
	plugins map[string]Plugin
}

// NewManager returns a new plugin Manager.
func NewManager() *Manager { return &Manager{plugins: make(map[string]Plugin)} }

// RegisterPlugin registers the plugin.
func (m *Manager) RegisterPlugin(p Plugin) *Manager {
	m.lock.Lock()
	defer m.lock.Unlock()

	name := p.Name()
	if _, ok := m.plugins[name]; ok {
		panic(fmt.Errorf("the plugin named '%s' has been registered", name))
	}
	m.plugins[name] = p
	return m
}

// UnregisterPlugin unregisters the plugin named pname.
func (m *Manager) UnregisterPlugin(pname string) *Manager {
	m.lock.Lock()
	delete(m.plugins, pname)
	m.lock.Unlock()
	return m
}

// Plugin returns the plugin by the name. Return nil instead if not exist.
func (m *Manager) Plugin(pname string) Plugin {
	m.lock.RLock()
	p := m.plugins[pname]
	m.lock.RUnlock()
	return p
}

// Plugins returns all the registered plugins.
func (m *Manager) Plugins() Plugins {
	m.lock.RLock()
	plugins := make(Plugins, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	m.lock.RUnlock()
	return plugins
}
