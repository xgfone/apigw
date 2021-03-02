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
	"log"
	"reflect"
	"strings"

	"github.com/xgfone/apigw/forward"
	"github.com/xgfone/apigw/plugin"
	"github.com/xgfone/ship/v3"
)

// RoutePluginConfig is used to configure the route plugin.
type RoutePluginConfig struct {
	PluginName   string      `json:"plugin_name"`
	PluginConfig interface{} `json:"plugin_config,omitempty"`
}

func (rpc1 RoutePluginConfig) equal(rpc2 RoutePluginConfig) bool {
	return rpc1.PluginName == rpc2.PluginName &&
		reflect.DeepEqual(rpc1.PluginConfig, rpc2.PluginConfig)
}

// Route represents a route.
type Route struct {
	// Required
	Host   string `json:"host,omitempty" validate:"zero|hostname_rfc1123"`
	Path   string `json:"path" validate:"required"`
	Method string `json:"method" validate:"required"`

	// Optional
	Forwarder     forward.Forwarder   `json:"-"`
	PluginConfigs []RoutePluginConfig `json:"plugin_configs,omitempty"`

	handler ship.Handler
}

// NewRoute returns a new Route.
func NewRoute(host, path, method string) Route {
	return Route{Host: host, Path: path, Method: method}
}

// Name returns the unified name of the route.
func (r Route) Name() string {
	return strings.Join([]string{r.Host, r.Method, r.Path}, "@")
}

func (r Route) routeKey() string {
	return strings.Join([]string{r.Method, r.Path}, "@")
}

func (r Route) equal(r2 Route) bool {
	if r.Host != r2.Host || r.Path != r2.Path || r.Method != r2.Method {
		return false
	}

	pc1len := len(r.PluginConfigs)
	pc2len := len(r2.PluginConfigs)
	if pc1len != pc2len {
		return false
	}

	for i := 0; i < pc1len; i++ {
		if !r.PluginConfigs[i].equal(r2.PluginConfigs[i]) {
			return false
		}
	}

	return true
}

// GetHosts returns the list of all the host domains.
func (g *Gateway) GetHosts() (hosts []string) {
	g.lock.RLock()
	hosts = make([]string, 0, len(g.routes))
	for host := range g.routes {
		hosts = append(hosts, host)
	}
	g.lock.RUnlock()
	return
}

// HasHost reports whether the host domain has been registered.
func (g *Gateway) HasHost(host string) bool {
	g.lock.RLock()
	_, ok := g.routes[host]
	g.lock.RUnlock()
	return ok
}

// AddHost adds the host domain.
func (g *Gateway) AddHost(host string) (err error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	if _, ok := g.routes[host]; ok {
		return
	} else if _, err = g.router.AddHost(host, nil); err == nil {
		g.routes[host] = make(map[string]Route)
	}

	return
}

// DelHost deletes the host domain and its routes.
func (g *Gateway) DelHost(host string) (err error) {
	if host == "" {
		return
	}

	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[host]; ok {
		for _, r := range routes {
			if err := r.Forwarder.Close(); err != nil {
				log.Printf("fail to close the forwarder '%s': %v\n", r.Forwarder.Name(), err)
			}
		}
	}

	return
}

// GetRoutes returns all the routes in the host domain.
func (g *Gateway) GetRoutes(host string) (routes []Route) {
	g.lock.RLock()
	if rs, exist := g.routes[host]; exist {
		routes = make([]Route, 0, len(rs))
		for _, route := range rs {
			routes = append(routes, route)
		}
	}
	g.lock.RUnlock()
	return
}

// GetRoute returns the full route.
func (g *Gateway) GetRoute(host, path, method string) (route Route, ok bool) {
	if path == "" {
		return
	}

	key := Route{Host: host, Path: path, Method: method}.routeKey()
	g.lock.RLock()
	if routes, exist := g.routes[host]; exist {
		route, ok = routes[key]
	}
	g.lock.RUnlock()
	return
}

// RegisterRoute registers the route and returns it.
//
// Notice: If the route has been registered, returns the registered route.
func (g *Gateway) RegisterRoute(route Route) (r Route, err error) {
	if route.Forwarder == nil {
		return Route{}, fmt.Errorf("forward handler must not be nil")
	} else if route.Path == "" {
		return Route{}, fmt.Errorf("the path must not be empty")
	}

	// Build the route plugins.
	plugins := make(plugin.Plugins, len(route.PluginConfigs))
	for i, pc := range route.PluginConfigs {
		plugin := g.Plugin(pc.PluginName)
		if plugin == nil {
			err = fmt.Errorf("no the pre-route plugin named '%s'", pc.PluginName)
			return
		}
		plugins[i] = plugin
	}
	plugins.Sort()

	// Build the route handler from plugins.
	route.handler = g.handleRequest
	for i, plugin := range plugins {
		mw, err := plugin.Plugin(route.PluginConfigs[i].PluginConfig)
		if err != nil {
			return Route{}, err
		}
		route.handler = mw(route.handler)
	}

	key := route.routeKey()

	g.lock.Lock()
	defer g.lock.Unlock()

	routes, ok := g.routes[route.Host]
	if ok {
		if r, ok := routes[key]; ok {
			return r, nil
		}
	}

	if err = g.addRoute(route); err != nil {
		return
	}

	if ok {
		routes[key] = route
	} else {
		g.routes[route.Host] = map[string]Route{key: route}
	}

	return route, nil
}

// UnregisterRoute unregisters the route, and returns the registered real route.
//
// Notice: It only needs the fields of host, path and method,
// and the others are ignored.
func (g *Gateway) UnregisterRoute(route Route) (r Route, err error) {
	if route.Path == "" {
		return Route{}, fmt.Errorf("the path must not be empty")
	}

	key := route.routeKey()
	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[route.Host]; ok {
		if r, ok = routes[key]; ok {
			if err = g.delRoute(route); err == nil {
				delete(routes, key)
				if err := r.Forwarder.Close(); err != nil {
					log.Printf("fail to close the forwarder '%s': %v\n", r.Forwarder.Name(), err)
				}
			}
		}
	}

	return
}

func (g *Gateway) addRoute(route Route) error {
	return g.router.AddRoute(ship.RouteInfo{
		Host:    route.Host,
		Path:    route.Path,
		Method:  route.Method,
		Handler: route.handler,
		CtxData: route,
	})
}

func (g *Gateway) delRoute(route Route) error {
	return g.router.DelRoute(ship.RouteInfo{
		Host:   route.Host,
		Path:   route.Path,
		Method: route.Method,
	})
}
