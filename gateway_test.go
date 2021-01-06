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
	"net/http"
	"strings"
	"time"

	"github.com/xgfone/apigw/forward"
	"github.com/xgfone/apigw/forward/endpoint"
	"github.com/xgfone/apigw/plugin"
	"github.com/xgfone/ship/v3"
)

func gwMiddleware(name string) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			// TODO: modity Method, Host or Path to shape the request.
			ctx.Logger().Infof("%s before", name)
			err := next(ctx)
			ctx.Logger().Infof("%s after", name)
			return err
		}
	}
}

func newTokenPlugin(config interface{}) (Middleware, error) {
	token := config.(string)
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			auth := strings.TrimSpace(ctx.GetHeader(ship.HeaderAuthorization))
			if auth == "" {
				return ship.ErrUnauthorized
			} else if index := strings.IndexByte(auth, ' '); index < 0 {
				return ship.ErrUnauthorized.Newf("invalid auth '%s'", auth)
			} else if authType := strings.TrimSpace(auth[:index]); authType != "token" {
				return ship.ErrUnauthorized.Newf("invalid auth type '%s'", authType)
			} else if authToken := strings.TrimSpace(auth[index+1:]); authToken != token {
				return ship.ErrUnauthorized.Newf("invalid auth token '%s'", authToken)
			}

			return next(ctx)
		}
	}, nil
}

func newLogPlugin(config interface{}) (Middleware, error) {
	return func(next Handler) Handler {
		return func(ctx *Context) error {
			ctx.Logger().Infof("request from '%s'", ctx.RemoteAddr())
			return next(ctx)
		}
	}, nil
}

func newPanicPlugin(config interface{}) (Middleware, error) {
	return func(next Handler) Handler {
		return func(ctx *Context) error { panic(next(ctx)) }
	}, nil
}

func ExampleGateway() {
	token := "authentication_token"

	gw := NewGateway()
	gw.RegisterMiddlewares(gwMiddleware("middleware1"), gwMiddleware("middleware2"))
	gw.RoutePluginManager().
		RegisterPlugin(plugin.NewPlugin("panic", 3, newPanicPlugin)).
		RegisterPlugin(plugin.NewPlugin("token", 1, newTokenPlugin)).
		RegisterPlugin(plugin.NewPlugin("log", 2, newLogPlugin))

	// Register the route and its backends.
	httpClient := endpoint.NewDefaultHTTPClient(100, time.Minute)
	backend1, _ := endpoint.NewHTTPEndpoint("", "http://127.0.0.1:8001/:path", httpClient)
	backend2, _ := endpoint.NewHTTPEndpoint("", "http://127.0.0.1:8002/:path", httpClient)

	forwarder := forward.NewLBForwarder(time.Minute)
	forwarder.EndpointManager().AddEndpoint(backend1)
	forwarder.EndpointManager().AddEndpoint(backend2)
	gw.RegisterRoute(Route{
		Host:      "www.example.com",
		Name:      "test",
		Path:      "/v1/:path",
		Method:    http.MethodGet,
		Forwarder: forwarder,
		PluginConfigs: []RoutePluginConfig{
			{PluginName: "token", PluginConfig: token},
			{PluginName: "log"},

			/// We don't configure the panic plugin for the current route,
			/// so it won't be used when triggering the route.
			// {PluginName: "panic"},
		},
	})

	// Start HTTP server.
	gw.Router().Start("127.0.0.1:80").Wait()

	////// Send the http request to test api gateway.
	//
	// $ curl -i http://127.0.0.1:12345/v1/test -H 'Host: www.example.com'
	// HTTP/1.1 401 Unauthorized
	// Date: Wed, 06 Jan 2021 21:21:25 GMT
	// Content-Length: 0
	//
	//
	// $ curl -i http://127.0.0.1:12345/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Wed, 06 Jan 2021 21:21:56 GMT
	// Server: MTI3LjAuMC4x
	//
	// {"backend":"127.0.0.1:8002"}
	//
	//
	// $ curl -i http://127.0.0.1:12345/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Wed, 06 Jan 2021 21:21:59 GMT
	// Server: MTI3LjAuMC4x
	//
	// {"backend":"127.0.0.1:8001"}
	//
	//
}
