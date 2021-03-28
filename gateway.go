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

// Package apigw implements the instance of the api gateway.
package apigw

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/xgfone/ship/v3"
	"github.com/xgfone/ship/v3/router"
	"github.com/xgfone/ship/v3/router/echo"
)

var gateways map[string]*Gateway

// RegisterGateway registers the gateway and returns true if successfully,
// or returns false if the gateway has been registered.
//
// Notice: you can use this to manage all the global gateways.
func RegisterGateway(g *Gateway) (ok bool) {
	if _, ok = gateways[g.Name()]; !ok {
		gateways[g.Name()] = g
	}
	return !ok
}

// UnregisterGateway unregisters the gateway by the name.
func UnregisterGateway(name string) *Gateway {
	g, ok := gateways[name]
	if ok {
		delete(gateways, name)
	}
	return g
}

// GetGateway returns the registered gateway by the name.
//
// Return nil if the gateway does not exist.
func GetGateway(name string) *Gateway {
	return gateways[name]
}

// GetGateways returns all the registered gateways.
func GetGateways() []*Gateway {
	gs := make([]*Gateway, 0, len(gateways))
	for _, g := range gateways {
		gs = append(gs, g)
	}
	return gs
}

// DefalutMaxBodySize is the default maximum size of the request body.
var DefalutMaxBodySize = 8 << 20 // 8MB

// DefaultGateway is the default global Gateway.
var DefaultGateway = NewGateway()

// Define some type aliases.
type (
	// Context is the context of Handler.
	Context = ship.Context

	// Handler is the http handler to handle http request.
	Handler = ship.Handler

	// Middleware is used to shape the http request before routing.
	Middleware = ship.Middleware

	// HTTPServerError represents a http server error.
	HTTPServerError = ship.HTTPServerError
)

// Gateway is an api gateway.
type Gateway struct {
	// Context is the user-defined context data.
	Context interface{}

	mdws   []Middleware
	router *ship.Ship

	mlock sync.RWMutex
	mdata map[string]interface{}

	lock      sync.RWMutex
	routes    map[string]map[string]Route // map[Host]map[RouteKey]Route
	plugins   map[string]Plugin           // map[PluginName]Plugin
	notfounds map[string]Handler          // map[Host]Handler
	hostmdws  map[string][]Middleware     // map[Host][]Middleware
	notfound  Handler
	hasgmdw   uint32
	hashmdw   uint32
}

// NewGateway returns a new Gateway.
func NewGateway() *Gateway {
	g := &Gateway{
		mdata:     make(map[string]interface{}),
		routes:    make(map[string]map[string]Route, 128),
		plugins:   make(map[string]Plugin, 8),
		notfounds: make(map[string]Handler),
		hostmdws:  make(map[string][]Middleware),
		notfound:  ship.NotFoundHandler(),
	}
	g.router = ship.Default()
	g.router.Lock = new(sync.RWMutex)
	g.router.NotFound = g.NotFoundHandler
	g.router.HandleError = g.handleError
	g.router.MaxBodySize = DefalutMaxBodySize
	g.router.RouteExecutor = g.ExecuteRoute
	g.router.SetNewRouter(func() router.Router {
		return router.NewLockRouter(echo.NewRouter(nil, nil))
	})
	return g
}

// SetMetadataForce is the same as SetMetadata, but still updates the key
// if the key has existed.
func (g *Gateway) SetMetadataForce(key string, value interface{}) {
	if value == nil {
		panic("Gateway.SetMetadata: value is nil")
	}

	g.mlock.Lock()
	g.mdata[key] = value
	g.mlock.Unlock()
}

// SetMetadata sets the metadata with the key and value.
//
// If the key has existed, do nothing. If the value is nil, panic.
func (g *Gateway) SetMetadata(key string, value interface{}) (ok bool) {
	if value == nil {
		panic("Gateway.SetMetadata: value is nil")
	}

	g.mlock.Lock()
	if _, exist := g.mdata[key]; !exist {
		g.mdata[key] = value
		ok = true
	}
	g.mlock.Unlock()
	return
}

// DelMetadata deletes and returns the metadata by the key.
//
// If not exist, do nothing and return nil.
func (g *Gateway) DelMetadata(key string) (value interface{}) {
	g.mlock.Lock()
	value = g.mdata[key]
	delete(g.mdata, key)
	g.mlock.Unlock()
	return
}

// GetMetadata returns the metadata by the key. But return nil if not exist.
func (g *Gateway) GetMetadata(key string) (value interface{}) {
	g.mlock.RLock()
	value = g.mdata[key]
	g.mlock.RUnlock()
	return
}

// GetMetadatas returns all the metadatas.
func (g *Gateway) GetMetadatas() (ms map[string]interface{}) {
	g.mlock.RLock()
	ms = make(map[string]interface{}, len(g.mdata))
	for key, value := range g.mdata {
		ms[key] = value
	}
	g.mlock.RUnlock()
	return
}

// Name returns the name of the gateway.
func (g *Gateway) Name() string { return g.router.Name }

// SetName resets the name of the gateway. The default is empty.
func (g *Gateway) SetName(name string) { g.router.Name = name }

// SetMaxBodySize resets the maxinum size of the request body.
// And 0 represents no limit.
//
// Default: DefalutMaxBodySize
func (g *Gateway) SetMaxBodySize(maxSize int) { g.router.MaxBodySize = maxSize }

// SetDefaultHost sets the default host to the existed host.
func (g *Gateway) SetDefaultHost(host string) (err error) {
	if host == "" {
		err = fmt.Errorf("host is empty")
	} else if dhost, _ := g.router.GetDefaultRouter(); dhost == host {
		return
	} else if router := g.router.Router(host); router == nil {
		err = fmt.Errorf("no the host '%s'", host)
	} else {
		g.router.SetDefaultRouter(host, router)
	}
	return
}

// GetDefaultHost returns the default host, which is "" by default.
func (g *Gateway) GetDefaultHost() (host string) {
	host, _ = g.router.GetDefaultRouter()
	return
}

// ExecuteRoute executes the route, which will execute the middlewares,
// find the route by the method and path from the underlying router,
// then execute the route handler.
//
// Notice: it is used to configure the field RouteExecutor of the underlying
// router. In general, you don't have to reset it.
func (g *Gateway) ExecuteRoute(ctx *Context) error {
	hasgmdw := atomic.LoadUint32(&g.hasgmdw) == 1
	hashmdw := atomic.LoadUint32(&g.hashmdw) == 1
	if hasgmdw || hashmdw {
		return g.executeRouteWithMiddleware(ctx, hasgmdw, hashmdw)
	}
	return g.findAndExecuteRoute(ctx)
}

func (g *Gateway) executeRouteWithMiddleware(ctx *Context, hasgmdw, hashmdw bool) error {
	handler := g.findAndExecuteRoute

	// For the host middleware
	if hashmdw {
		var mdws []Middleware
		g.lock.RLock()
		mdws = g.hostmdws[ctx.RouteInfo.Host]
		for i := len(mdws) - 1; i >= 0; i-- {
			handler = mdws[i](handler)
		}

		mdws = g.hostmdws[ctx.Host()]
		for i := len(mdws) - 1; i >= 0; i-- {
			handler = mdws[i](handler)
		}
		g.lock.RUnlock()
	}

	// For the global middleware
	if hasgmdw {
		for i := len(g.mdws) - 1; i >= 0; i-- {
			handler = g.mdws[i](handler)
		}
	}

	return handler(ctx)
}

func (g *Gateway) findAndExecuteRoute(ctx *Context) error {
	return ctx.Execute()
}

func (g *Gateway) handleRequest(ctx *Context) error {
	// Forward the request to the backend server.
	return ctx.RouteCtxData.(Route).Forwarder.Forward(ctx)
}

func (g *Gateway) handleError(ctx *Context, err error) {
	if !ctx.IsResponded() {
		switch e := err.(type) {
		case ship.HTTPError:
			ctx.BlobText(e.Code, e.CT, e.Error())
		default:
			ctx.Text(http.StatusInternalServerError, err.Error())
		}
	}
}

// NotFoundHandler is the handler of the router to handle the NotFound.
//
// Notice: it is used to configure the field NotFound of the underlying router.
// In general, you don't have to reset it.
func (g *Gateway) NotFoundHandler(ctx *Context) error {
	g.lock.RLock()
	handler := g.notfound
	if h, ok := g.notfounds[ctx.Host()]; ok {
		handler = h
	} else if h, ok = g.notfounds[ctx.RouteInfo.Host]; ok {
		handler = h
	}
	g.lock.RUnlock()
	return handler(ctx)
}

// SetHostNotFound sets the NotFound handler of the host router.
//
// If the handler is nil, unset the setting of the NotFound handler of host.
func (g *Gateway) SetHostNotFound(host string, handler Handler) {
	g.lock.Lock()
	if handler == nil {
		delete(g.notfounds, host)
	} else {
		g.notfounds[host] = handler
	}
	g.lock.Unlock()
}

// SetDefaultNotFound sets the default NotFound handler.
func (g *Gateway) SetDefaultNotFound(notFound Handler) {
	if notFound == nil {
		panic("the NotFound handler must not be nil")
	}
	g.lock.Lock()
	g.notfound = notFound
	g.lock.Unlock()
}

// HostNotFounds returns the list of the NotFound handlers of all the host.
func (g *Gateway) HostNotFounds() map[string]Handler {
	g.lock.RLock()
	nfs := make(map[string]Handler, len(g.notfounds))
	for host, nf := range g.notfounds {
		nfs[host] = nf
	}
	g.lock.RUnlock()
	return nfs
}

// HostNotFound returns the NotFound handler of the host.
//
// If the NotFound handler does not exist, return nil.
func (g *Gateway) HostNotFound(host string) Handler {
	g.lock.RLock()
	nf := g.notfounds[host]
	g.lock.RUnlock()
	return nf
}

// Router returns the underlying http router.
func (g *Gateway) Router() *ship.Ship { return g.router }

// ServeHTTP implements the interface http.Handler.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.router.ServeHTTP(w, r)
}

// RegisterGlobalMiddlewares registers the global route middlewares,
// which will act on all the hosts.
//
// Notice: It must be called before starting the gateway.
func (g *Gateway) RegisterGlobalMiddlewares(mws ...Middleware) {
	g.mdws = append(g.mdws, mws...)
	if len(g.mdws) == 0 {
		atomic.StoreUint32(&g.hasgmdw, 0)
	} else {
		atomic.StoreUint32(&g.hasgmdw, 1)
	}
}

// ResetGlobalMiddlewares cleans the old global router middlewares
// and resets it to mws.
//
// Notice: It must be called before starting the gateway.
func (g *Gateway) ResetGlobalMiddlewares(mws ...Middleware) {
	g.mdws = append([]Middleware{}, mws...)
	if len(g.mdws) == 0 {
		atomic.StoreUint32(&g.hasgmdw, 0)
	} else {
		atomic.StoreUint32(&g.hasgmdw, 1)
	}
}

// RegisterHostMiddlewares registers the host route middlewares,
// which will act on the given host.
//
// Notice: It can be called at any time.
func (g *Gateway) RegisterHostMiddlewares(host string, mws ...Middleware) {
	if len(mws) == 0 {
		return
	}

	g.lock.Lock()
	g.hostmdws[host] = append(g.hostmdws[host], mws...)
	atomic.StoreUint32(&g.hashmdw, 1)
	g.lock.Unlock()
}

// ResetHostMiddlewares cleans the old host router middlewares
// and resets it to mws.
//
// Notice: It can be called at any time.
func (g *Gateway) ResetHostMiddlewares(host string, mws ...Middleware) {
	g.lock.Lock()
	if len(mws) == 0 {
		delete(g.hostmdws, host)
		if len(g.hostmdws) == 0 {
			atomic.StoreUint32(&g.hashmdw, 0)
		}
	} else {
		g.hostmdws[host] = append([]Middleware{}, mws...)
		atomic.StoreUint32(&g.hashmdw, 1)
	}
	g.lock.Unlock()
}

// RegisterPlugin registers the plugin.
func (g *Gateway) RegisterPlugin(p Plugin) (err error) {
	name := p.Name()
	g.lock.Lock()
	if _, ok := g.plugins[name]; ok {
		err = fmt.Errorf("the plugin named '%s' has been registered", name)
	} else {
		g.plugins[name] = p
	}
	g.lock.Unlock()
	return
}

// UnregisterPlugin unregisters the plugin named pname.
func (g *Gateway) UnregisterPlugin(pname string) {
	g.lock.Lock()
	delete(g.plugins, pname)
	g.lock.Unlock()
}

// Plugin returns the plugin by the name. Return nil instead if not exist.
func (g *Gateway) Plugin(pname string) Plugin {
	g.lock.RLock()
	p := g.plugins[pname]
	g.lock.RUnlock()
	return p
}

// Plugins returns all the registered plugins.
func (g *Gateway) Plugins() Plugins {
	g.lock.RLock()
	plugins := make(Plugins, 0, len(g.plugins))
	for _, p := range g.plugins {
		plugins = append(plugins, p)
	}
	g.lock.RUnlock()
	return plugins
}
