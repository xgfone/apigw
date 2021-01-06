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

	"github.com/xgfone/apigw/forward"
	"github.com/xgfone/apigw/plugin"
	"github.com/xgfone/ship/v3"
)

// RoutePluginConfig is used to configure the route plugin.
type RoutePluginConfig struct {
	PluginName   string
	PluginConfig interface{}
}

// Route represents a route.
type Route struct {
	Host   string
	Name   string
	Path   string
	Method string

	Forwarder     forward.Forwarder
	PluginConfigs []RoutePluginConfig
}

// RegisterRoute registers the route.
func (g *Gateway) RegisterRoute(route Route) error {
	if route.Forwarder == nil {
		return fmt.Errorf("forward handler must not be nil")
	}

	plugins := make(plugin.Plugins, len(route.PluginConfigs))
	for i, rpc := range route.PluginConfigs {
		plugin := g.plugins.Plugin(rpc.PluginName)
		if plugin == nil {
			return fmt.Errorf("no the pre-route plugin named '%s'", rpc.PluginName)
		}
		plugins[i] = plugin
	}
	plugins.Sort()

	handler := g.handleRequest
	for i, plugin := range plugins {
		mw, err := plugin.Plugin(route.PluginConfigs[i].PluginConfig)
		if err != nil {
			return err
		}

		handler = mw(handler)
	}

	return g.router.AddRoute(ship.RouteInfo{
		Host:    route.Host,
		Name:    route.Name,
		Path:    route.Path,
		Method:  route.Method,
		Handler: handler,
		CtxData: route,
	})
}

// UnregisterRoute unregisters the route.
func (g *Gateway) UnregisterRoute(route Route) error {
	return g.router.DelRoute(ship.RouteInfo{
		Host:   route.Host,
		Name:   route.Name,
		Path:   route.Path,
		Method: route.Method,
	})
}
