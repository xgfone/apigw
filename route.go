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
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"

	"github.com/xgfone/ship/v3"
)

// Predefine some errors.
var (
	ErrNoHost    = errors.New("no gateway host")
	ErrNoRoute   = errors.New("no gateway route")
	ErrEmptyPath = errors.New("the empty route path")
)

// RoutePlugin is used to configure the route plugin.
type RoutePlugin struct {
	Name   string      `json:"name" validate:"required"`
	Config interface{} `json:"config,omitempty"`
}

func (rpc1 RoutePlugin) equal(rpc2 RoutePlugin) bool {
	return rpc1.Name == rpc2.Name && reflect.DeepEqual(rpc1.Config, rpc2.Config)
}

// Forwarder is used to forward the http request to the backend server.
type Forwarder interface {
	Forward(ctx *Context) error
	Name() string
	io.Closer
}

// Route represents a route.
type Route struct {
	// Required
	Host   string `json:"host,omitempty" validate:"zero|hostname_rfc1123"`
	Path   string `json:"path" validate:"required"`
	Method string `json:"method" validate:"required"`

	// Optional
	Forwarder Forwarder     `json:"-"`
	Plugins   []RoutePlugin `json:"plugins,omitempty"`

	handler Handler
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

	pc1len := len(r.Plugins)
	pc2len := len(r2.Plugins)
	if pc1len != pc2len {
		return false
	}

	for i := 0; i < pc1len; i++ {
		if !r.Plugins[i].equal(r2.Plugins[i]) {
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
//
// If the host has been added, do nothing.
func (g *Gateway) AddHost(host string) (err error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	if _, ok := g.routes[host]; ok || host == "" {
		return
	} else if _, err = g.router.AddHost(host, nil); err == nil {
		g.routes[host] = make(map[string]Route)
	}

	return
}

// DelHost deletes the host domain and its routes.
//
// If the host does not exist, do nothing.
func (g *Gateway) DelHost(host string) (err error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[host]; ok {
		g.router.DelHost(host)
		for _, r := range routes {
			if err := r.Forwarder.Close(); err != nil {
				log.Printf("fail to close the forwarder '%s': %v\n", r.Forwarder.Name(), err)
			}
		}
	}

	return
}

// GetRoutes returns all the routes in the host domain.
//
// If the host does not exist, reutrn (nil, ErrNoHost).
func (g *Gateway) GetRoutes(host string) (routes []Route, err error) {
	g.lock.RLock()
	if rs, exist := g.routes[host]; exist {
		routes = make([]Route, 0, len(rs))
		for _, route := range rs {
			routes = append(routes, route)
		}
	} else {
		err = ErrNoHost
	}
	g.lock.RUnlock()
	return
}

// GetRoute returns the full route.
func (g *Gateway) GetRoute(host, path, method string) (route Route, err error) {
	if path == "" {
		return Route{}, ErrEmptyPath
	}

	key := Route{Host: host, Path: path, Method: method}.routeKey()
	g.lock.RLock()
	if routes, exist := g.routes[host]; !exist {
		err = ErrNoHost
	} else if route, exist = routes[key]; !exist {
		err = ErrNoRoute
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
		return Route{}, ErrEmptyPath
	}

	// Build the route plugins.
	plugins := make(Plugins, len(route.Plugins))
	for i, pc := range route.Plugins {
		plugin := g.Plugin(pc.Name)
		if plugin == nil {
			err = fmt.Errorf("no the pre-route plugin named '%s'", pc.Name)
			return
		}
		plugins[i] = plugin
	}
	plugins.Sort()

	// Build the route handler from plugins.
	route.handler = g.handleRequest
	for i, plugin := range plugins {
		mw, err := plugin.Plugin(route.Plugins[i].Config)
		if err != nil {
			return Route{}, err
		}
		route.handler = mw(route.handler)
	}

	key := route.routeKey()

	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[route.Host]; !ok {
		return Route{}, ErrNoHost
	} else if r, ok = routes[key]; ok {
		return
	} else if err = g.addRoute(route); err != nil {
		return
	} else {
		routes[key] = route
	}

	return route, nil
}

// UnregisterRoute unregisters the route, and returns the registered real route.
//
// Notice: It only needs the fields of host, path and method,
// and the others are ignored.
func (g *Gateway) UnregisterRoute(route Route) (r Route, err error) {
	if route.Path == "" {
		return Route{}, ErrEmptyPath
	}

	key := route.routeKey()
	g.lock.Lock()
	defer g.lock.Unlock()

	routes, ok := g.routes[route.Host]
	if !ok {
		return Route{}, ErrNoHost
	} else if r, ok = routes[key]; !ok {
		return Route{}, ErrNoRoute
	}

	if err = g.delRoute(route); err != nil {
		return
	}

	delete(routes, key)
	if err := r.Forwarder.Close(); err != nil {
		log.Printf("fail to close the forwarder '%s': %v\n", r.Forwarder.Name(), err)
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
