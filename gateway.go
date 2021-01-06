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
	"net/http"
	"time"

	"github.com/xgfone/apigw/plugin"
	"github.com/xgfone/ship/v3"
)

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
	mdws    []Middleware
	router  *ship.Ship
	plugins *plugin.Manager
	timeout time.Duration
}

// NewGateway returns a new Gateway.
func NewGateway() *Gateway {
	g := &Gateway{router: ship.Default(), plugins: plugin.NewManager()}
	g.router.Pre(g.handleMiddleware)
	return g
}

// Router returns the underlying http router.
func (g *Gateway) Router() *ship.Ship { return g.router }

// ServeHTTP implements the interface http.Handler.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.router.ServeHTTP(w, r)
}

func (g *Gateway) handleMiddleware(next Handler) Handler {
	return func(ctx *Context) (err error) {
		_len := len(g.mdws)
		if _len == 0 {
			return next(ctx)
		}

		handler := next
		for i := _len - 1; i >= 0; i-- {
			handler = g.mdws[i](handler)
		}
		return handler(ctx)
	}
}

func (g *Gateway) handleRequest(ctx *Context) error {
	// Forward the request to the backend server.
	return ctx.RouteCtxData.(Route).Forwarder.Forward(ctx)
}

// RegisterMiddlewares registers some router middlewares, which are run
// before routing, so you can modify the http request by using it
// to make a difference when routing later.
//
// Notice: it only uses Host, Method and Path to route the request.
func (g *Gateway) RegisterMiddlewares(mws ...Middleware) {
	g.mdws = append(g.mdws, mws...)
}

// ResetMiddlewares cleans the old and resets the router middleware to mws.
func (g *Gateway) ResetMiddlewares(mws ...Middleware) {
	g.mdws = append([]Middleware{}, mws...)
}

// RoutePluginManager returns the manager of the route plugin.
func (g *Gateway) RoutePluginManager() *plugin.Manager { return g.plugins }
