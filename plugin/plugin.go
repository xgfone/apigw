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

// Package plugin defines the interface of the plugin and the manager.
package plugin

import (
	"fmt"
	"sort"

	"github.com/xgfone/ship/v3"
)

// Plugin represents a plugin to handle the request.
type Plugin interface {
	fmt.Stringer
	Name() string
	Priority() int
	Plugin(config interface{}) (ship.Middleware, error)
}

// Plugins is a set of Plugins.
type Plugins []Plugin

// Sort sorts itself.
func (ps Plugins) Sort()              { sort.Sort(ps) }
func (ps Plugins) Len() int           { return len(ps) }
func (ps Plugins) Less(i, j int) bool { return ps[i].Priority() < ps[j].Priority() }
func (ps Plugins) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

// NewPlugin returns a new plugin.
func NewPlugin(name string, prio int, newPlugin func(interface{}) (ship.Middleware, error)) Plugin {
	return pluginer{name: name, prio: prio, newPlugin: newPlugin}
}

type pluginer struct {
	prio      int
	name      string
	newPlugin func(interface{}) (ship.Middleware, error)
}

func (p pluginer) Priority() int  { return p.prio }
func (p pluginer) Name() string   { return p.name }
func (p pluginer) String() string { return fmt.Sprintf("Plugin(name=%s)", p.name) }
func (p pluginer) Plugin(config interface{}) (ship.Middleware, error) {
	return p.newPlugin(config)
}
