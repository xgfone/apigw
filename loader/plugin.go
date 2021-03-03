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

package loader

import (
	"fmt"

	"github.com/xgfone/apigw"
)

// PluginLoader is used to load the plugin.
type PluginLoader interface {
	Name() string
	Plugin() (apigw.Plugin, error)
}

type pluginLoader struct {
	name   string
	plugin func() (apigw.Plugin, error)
}

// NewPluginLoader returns a new plugin loader.
func NewPluginLoader(name string, plugin func() (apigw.Plugin, error)) PluginLoader {
	return pluginLoader{name: name, plugin: plugin}
}

func (l pluginLoader) Name() string                  { return l.name }
func (l pluginLoader) Plugin() (apigw.Plugin, error) { return l.plugin() }

var pluginLoaders = make(map[string]PluginLoader, 4)

// RegisterPluginLoader registers the plugin loader.
//
// If the loader has been registered, it will panic.
func RegisterPluginLoader(l PluginLoader) {
	name := l.Name()
	if _, ok := pluginLoaders[name]; ok {
		panic(fmt.Errorf("the plugin loader named '%s' has been registered", name))
	}
	pluginLoaders[name] = l
}

// UnregisterPluginLoader unregisters the plugin loader by the name.
func UnregisterPluginLoader(name string) { delete(pluginLoaders, name) }

// GetPluginLoader returns the plugin loader by its name.
//
// Return nil if it does not exist.
func GetPluginLoader(name string) PluginLoader { return pluginLoaders[name] }

// GetPluginLoaders returns the name list of all the plugin loaders.
func GetPluginLoaders() []string {
	names := make([]string, 0, len(pluginLoaders))
	for name := range pluginLoaders {
		names = append(names, name)
	}
	return names
}
