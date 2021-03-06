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

package apigw_test

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/apigw/forward/lb/backend"
	"github.com/xgfone/ship/v4"
)

func gwMiddleware(name string) apigw.Middleware {
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) error {
			// TODO: modity Method, Host or Path to shape the request.
			ctx.Logger().Infof("%s before", name)
			err := next(ctx)
			ctx.Logger().Infof("%s after", name)
			return err
		}
	}
}

func newTokenAuthPlugin(config interface{}) (apigw.Middleware, error) {
	token := config.(string)
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) error {
			auth := strings.TrimSpace(ctx.GetHeader(ship.HeaderAuthorization))
			if auth == "" {
				return ship.ErrUnauthorized
			} else if index := strings.IndexByte(auth, ' '); index < 0 {
				return ship.ErrUnauthorized.Newf("invalid auth '%s'", auth)
			} else if authType := strings.TrimSpace(auth[:index]); authType != "token" {
				return ship.ErrUnauthorized.Newf("invalid auth type '%s'", authType)
			} else if authToken := strings.TrimSpace(auth[index+1:]); authToken != token {
				return ship.ErrForbidden.Newf("invalid auth token '%s'", authToken)
			}

			return next(ctx)
		}
	}, nil
}

func newLogPlugin(config interface{}) (apigw.Middleware, error) {
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) error {
			ctx.Logger().Infof("request from '%s'", ctx.RemoteAddr())
			return next(ctx)
		}
	}, nil
}

func newPanicPlugin(config interface{}) (apigw.Middleware, error) {
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) error { panic(next(ctx)) }
	}, nil
}

func ExampleGateway() {
	host := "www.example.com" // Or the regexp host like `[a-zA-z0-9]+\.example\.com`

	// Initialize the gateway.
	gw := lb.NewGateway() // We use lb.Gateway instead of apigw.Gateway.

	// (Optional): You can set the customized logger.
	// gw.Router().SetLogger(logger)

	// (Optional): Customize the host router manager based on the regular expression.
	//             The default implementation is based on the stdlib "regexp".
	// gw.Router().SetNewRegexpHostRouter(newRegexpHostRouterFunc)

	// Register the middlewares and the plugins.
	gw.RegisterGlobalMiddlewares(gwMiddleware("middleware1"), gwMiddleware("middleware2"))
	gw.RegisterPlugin(apigw.NewPlugin("panic", 3, newPanicPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("token", 1, newTokenAuthPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("log", 2, newLogPlugin))

	// Add the host domain before registering the route.
	gw.AddHost(host)

	// (Optional): Set the middlewares only for the given host.
	gw.RegisterHostMiddlewares(host, gwMiddleware(host))

	// (Optional): Set the NotFound handler for a certain host domain.
	gw.SetHostNotFound(host, func(c *apigw.Context) error {
		return c.Text(404, "no route: host=%s, method=%s, path=%s", host, c.Method(), c.Path())
	})

	// Create some backend endpoints, such as HTTP, GRPC, etc.
	backend1, _ := backend.NewHTTPBackend("127.0.0.1:8001", nil)
	backend2, _ := backend.NewHTTPBackend("127.0.0.1:8002", nil)
	backend3, _ := backend.NewHTTPBackend("127.0.0.1:8003", nil)
	backend4, _ := backend.NewHTTPBackend("127.0.0.1:8004", nil)

	// Create the route.
	route := apigw.NewRoute(host, "/v1/:path", http.MethodGet)
	forwarder := lb.NewForwarder(route.Name())
	forwarder.SetSessionTimeout(time.Second * 30) // For session stick timeout

	// Add the backends specific to the route
	forwarder.AddBackendWithChecker(backend1, nil, lb.BackendCheckerDurationZero)
	forwarder.AddBackendWithChecker(backend2, nil, lb.BackendCheckerDurationZero)

	route.Forwarder = forwarder          // Set the backend forwarder for the route
	route.Plugins = []apigw.RoutePlugin{ // Set the plugins which the route will use
		{Name: "token", Config: "authentication_token"},
		{Name: "log"},

		/// We don't configure the panic plugin for the current route,
		/// so it won't be used when triggering the route.
		// {Name: "panic"},
	}

	// Register the route into the gateway.
	if _, err := gw.RegisterRoute(route); err != nil {
		fmt.Printf("fail to register the route '%s': %v\n", route.Name(), err)
		return
	}

	// Backends can be added after the route is registered.
	// And the backend supports the group, which is also separated by the host.
	group := gw.GetBackendGroupManager(host).AddOrNewBackendGroup("group_name")

	// 1. Support to add the backend into the group before adding the group backend into the route forwarder.
	group.AddBackendWithChecker(backend3, nil, lb.BackendCheckerDurationZero)

	// 2. Add the backend group into the route forwarder.
	forwarder.AddBackendGroup(group)

	// 3. Support to add the backend into the group after adding the group backend into the route forwarder.
	group.AddBackendWithChecker(backend4, nil, lb.BackendCheckerDurationZero)

	// Start HTTP server.
	gw.Router().Start("127.0.0.1:80").Wait()

	// TEST THE GATEWAY:
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com'
	// HTTP/1.1 401 Unauthorized
	// Date: Fri, 25 Jun 2021 12:22:21 GMT
	// Content-Length: 12
	// Content-Type: text/plain; charset=utf-8
	//
	// Unauthorized
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Fri, 25 Jun 2021 12:23:20 GMT
	// X-Server-Id: MTI3LjAuMC4xOjgwMDM=
	//
	// {"backend":"127.0.0.1:8003"}
	//
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Fri, 25 Jun 2021 12:24:57 GMT
	// X-Server-Id: MTI3LjAuMC4xOjgwMDE=
	//
	// {"backend":"127.0.0.1:8001"}
	//
	//
	// $ curl -i http://127.0.0.1:80/v2/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 404 Not Found
	// Content-Type: text/plain; charset=UTF-8
	// Date: Fri, 25 Jun 2021 12:25:43 GMT
	// Content-Length: 57
	//
	// no route: host=www.example.com, method=GET, path=/v2/test
	//
}
