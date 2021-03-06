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

	"github.com/xgfone/ship/v4"
)

// Predefine some errors.
var (
	ErrNoHost         = errors.New("no gateway host")
	ErrNoRoute        = errors.New("no gateway route")
	ErrNoBackendGroup = errors.New("no backend group")
	ErrExistedRoute   = errors.New("the existed route")
	ErrEmptyPath      = errors.New("the empty route path")
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
	Forward(*Context) error
	Name() string
	io.Closer
}

// Matcher is used to match the host route.
type Matcher struct {
	Host   string `json:"host,omitempty" validate:"zero|hostname_rfc1123"`
	Path   string `json:"path" validate:"required"`
	Method string `json:"method" validate:"required"`
}

// Route represents a route.
type Route struct {
	Matcher   Matcher       `json:"matcher"`
	Plugins   []RoutePlugin `json:"plugins,omitempty"`
	Forwarder Forwarder     `json:"-"`
	handler   Handler
}

func (m Matcher) routeKey() string {
	return strings.Join([]string{m.Method, m.Path}, "@")
}

// Name returns the unified name of the route, which indicates a unique route.
func (m Matcher) Name() string {
	return strings.Join([]string{m.Host, m.Method, m.Path}, "@")
}

// NewMatcher returns a new Matcher.
func NewMatcher(host, path, method string) Matcher {
	return Matcher{Host: host, Path: path, Method: method}
}

// NewRoute is equal to NewRouteWithMatcher(NewMatcher(host, path, method)).
func NewRoute(host, path, method string) Route {
	return Route{Matcher: NewMatcher(host, path, method)}
}

// NewRoute returns a new Route with the matcher.
func NewRouteWithMatcher(m Matcher) Route { return Route{Matcher: m} }

// Name returns the unified name of the route, which indicates a unique route.
func (r Route) Name() string { return r.Matcher.Name() }

// Equal reports whether the route is eqaul to other.
func (r Route) Equal(other Route) bool {
	if r.Matcher != other.Matcher {
		return false
	}

	pc1len := len(r.Plugins)
	pc2len := len(other.Plugins)
	if pc1len != pc2len {
		return false
	}

	for i := 0; i < pc1len; i++ {
		if !r.Plugins[i].equal(other.Plugins[i]) {
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

	if _, ok := g.routes[host]; ok {
		return
	} else if host == "" {
		g.routes[host] = make(map[string]Route)
	} else if _, err = g.router.AddHost(host, nil); err == nil {
		g.routes[host] = make(map[string]Route)
	}

	return
}

// DelHost deletes the host domain and its routes, which also cleans
// the middlewares and NotFound handlers associated with the host domain.
//
// If the host does not exist, do nothing.
func (g *Gateway) DelHost(host string) (err error) {
	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[host]; ok {
		delete(g.routes, host)
		delete(g.hostmdws, host)
		delete(g.notfounds, host)
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

	key := Matcher{Host: host, Path: path, Method: method}.routeKey()
	g.lock.RLock()
	if routes, exist := g.routes[host]; !exist {
		err = ErrNoHost
	} else if route, exist = routes[key]; !exist {
		err = ErrNoRoute
	}
	g.lock.RUnlock()
	return
}

// RegisterRoute registers the route and returns it. But it will return
// the registered route and ErrExistedRoute, if the route has been registered.
//
// Notice: Before registering the route, you must add the corresponding host.
// Or return ErrNoHost.
func (g *Gateway) RegisterRoute(route Route) (r Route, err error) {
	if route.Forwarder == nil {
		return Route{}, fmt.Errorf("forward handler must not be nil")
	} else if route.Matcher.Path == "" {
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

	key := route.Matcher.routeKey()

	g.lock.Lock()
	defer g.lock.Unlock()

	if routes, ok := g.routes[route.Matcher.Host]; !ok {
		return Route{}, ErrNoHost
	} else if r, ok = routes[key]; ok {
		err = ErrExistedRoute
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
// If the host does not exist, return ErrNoHost.
// If the route is not registered, return ErrNoRoute.
//
// Notice: It only needs the fields of host, path and method,
// and the others are ignored.
func (g *Gateway) UnregisterRoute(m Matcher) (r Route, err error) {
	if m.Path == "" {
		return Route{}, ErrEmptyPath
	}

	key := m.routeKey()
	g.lock.Lock()
	defer g.lock.Unlock()

	routes, ok := g.routes[m.Host]
	if !ok {
		return Route{}, ErrNoHost
	} else if r, ok = routes[key]; !ok {
		return Route{}, ErrNoRoute
	}

	if err = g.delRoute(m); err != nil {
		return
	}

	delete(routes, key)
	if err := r.Forwarder.Close(); err != nil {
		log.Printf("fail to close the forwarder '%s': %v\n", r.Forwarder.Name(), err)
	}

	return
}

func (g *Gateway) addRoute(r Route) error {
	return g.router.AddRoute(ship.RouteInfo{
		Host:    r.Matcher.Host,
		Path:    r.Matcher.Path,
		Method:  r.Matcher.Method,
		Handler: r.handler,
		CtxData: r,
	})
}

func (g *Gateway) delRoute(m Matcher) error {
	return g.router.DelRoute(ship.RouteInfo{
		Host:   m.Host,
		Path:   m.Path,
		Method: m.Method,
	})
}
