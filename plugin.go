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

package apigw

import (
	"fmt"
	"sort"
)

// Plugin represents a plugin to handle the request.
type Plugin interface {
	// The description of the plugin, such as "Plugin(name=XXX)".
	fmt.Stringer

	// The name of the plugin, such as "XXX".
	Name() string

	// The bigger the value, the higher the priority, and the plugin is called preferentially.
	Priority() int

	// Create a new middleware to execute the plugin when triggering the route.
	Plugin(config interface{}) (Middleware, error)
}

// Plugins is a set of Plugins.
type Plugins []Plugin

// Sort sorts itself.
func (ps Plugins) Sort()              { sort.Stable(ps) }
func (ps Plugins) Len() int           { return len(ps) }
func (ps Plugins) Less(i, j int) bool { return ps[i].Priority() < ps[j].Priority() }
func (ps Plugins) Swap(i, j int)      { ps[i], ps[j] = ps[j], ps[i] }

// NewPlugin returns a new plugin.
func NewPlugin(name string, prio int, newf func(interface{}) (Middleware, error)) Plugin {
	return plugin{name: name, prio: prio, newf: newf}
}

type plugin struct {
	prio int
	name string
	newf func(interface{}) (Middleware, error)
}

func (p plugin) Priority() int  { return p.prio }
func (p plugin) Name() string   { return p.name }
func (p plugin) String() string { return fmt.Sprintf("Plugin(name=%s)", p.name) }
func (p plugin) Plugin(config interface{}) (Middleware, error) {
	return p.newf(config)
}
