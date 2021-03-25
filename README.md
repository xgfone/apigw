# apigw [![Build Status](https://travis-ci.org/xgfone/apigw.svg?branch=master)](https://travis-ci.org/xgfone/apigw) [![GoDoc](https://godoc.org/github.com/xgfone/apigw?status.svg)](https://pkg.go.dev/github.com/xgfone/apigw) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/apigw/master/LICENSE)

Another simple, flexible, high performance api gateway library implemented by Go(**`≥go1.15`**). For the binary program, see the another repository [apigateway](https://github.com/xgfone/apigateway).


### Features
- Flexible, high performance and zero memory allocation for the core engine. See Benchmark, [Example](#example) and [apigateway](https://github.com/xgfone/apigateway).
- Support the virtual host, and different hosts has their own independent routes and NotFound.
- Support the health check for the backend, that's upstream server.
- Support the group of the upstream servers as the backend.
- Support to customize the backend forwarder of the route.
- Most of the functions are implemented by the plugin mode.
- Too few core engine codes, ~1400 lines.
    ```shell
    $ cloc --exclude-dir=plugins --not-match-f=_test.go --include-lang=Go --quiet .
    -------------------------------------------------------------------------------
    Language                     files          blank        comment           code
    -------------------------------------------------------------------------------
    Go                              17            323            530           1408
    -------------------------------------------------------------------------------
    SUM:                            17            323            530           1408
    -------------------------------------------------------------------------------
    ```


### Difference between Middleware and Plugin
- Plugin is pluggable during running, and run after routing the request.
- Middleware is unpluggable after running, and run before routing the request.

Unless you want to modify the request to affect routing, it is recommended to use the plugins instead of the middlewares. Because plugins performs better than middlewares. To log all the requests to all the host domains, you maybe use the global middleware (or the plugin) to finish it.

**Notice:** The framework is based on [ship](https://github.com/xgfone/ship), so it's based on `Path` and `Method` of the request URL to route the request at first, then the api gateway framework takes over the handling and forwards it to one of the backends, such as routing based on the header or rewriting the request.


### TODO List
- [ ] Add some authentications.
- [ ] Add some built-in plugins and middlewares.
- [ ] Others.


## Install
```shell
$ go get -u github.com/xgfone/apigw
```


## Example
```go
package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/apigw/forward/lb/backend"
	slb "github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/goapp"
	"github.com/xgfone/goapp/log"
	"github.com/xgfone/ship/v3"
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

func main() {
	host := "www.example.com" // Or the regexp host like `[a-zA-z0-9]+\.example\.com`

	// Create the global health checker of the backend endpoint, that's, upstream server.
	// But you can also ignore it and do not to use it.
	healthChecker := slb.NewHealthCheck()
	defer healthChecker.Stop()

	// Parse the CLI arguments and the configuration file,
	// and initialize the logging.
	goapp.Init("")

	// Initialize the gateway.
	gw := lb.NewGateway() // We use lb.Gateway instead of apigw.Gateway.

	// You can set the customized logger.
	// gw.Router().SetLogger(logger)

	// (Optional) Customize the host router manager based on the regular expression.
	// The default implementation is based on the stdlib "regexp".
	// gw.Router().SetNewRegexpHostRouter(newRegexpHostRouterFunc)

	// Register the middlewares and the plugins.
	gw.RegisterGlobalMiddlewares(gwMiddleware("middleware1"), gwMiddleware("middleware2"))
	gw.RegisterPlugin(apigw.NewPlugin("panic", 3, newPanicPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("token", 1, newTokenAuthPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("log", 2, newLogPlugin))

	// Add the host domain before registering the route.
	gw.AddHost(host)

	// (Optional) Set the middlewares only for the given host.
	gw.RegisterHostMiddlewares(host, gwMiddleware(host))

	// (Optional) Set the NotFound handler for a certain host domain.
	gw.SetHostNotFound(host, func(c *apigw.Context) error {
		return c.Text(404, "no route: host=%s, method=%s, path=%s", host, c.Method(), c.Path())
	})

	// Create some backend endpoints, such as HTTP, GRPC, etc.
	backend1, _ := backend.NewHTTPBackend("127.0.0.1:8001", nil)
	backend2, _ := backend.NewHTTPBackend("127.0.0.1:8002", nil)

	// Create the route.
	route := apigw.NewRoute(host, "/v1/:path", http.MethodGet)
	forwarder := lb.NewForwarder(route.Name())
	forwarder.HealthCheck = healthChecker                   // For the health check of backends, or not use it
	forwarder.SetSessionTimeout(time.Second * 30)           // For session stick timeout, or not use
	forwarder.SetSession(slb.NewMemorySession(time.Minute)) // For session stick manager, or not use
	forwarder.AddBackends(backend1, backend2)               // Add the backends specific to the route
	route.Forwarder = forwarder                             // Set the backend forwarder for the route
	route.Plugins = []apigw.RoutePlugin{                    // Set the plugins which the route will use
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
	group := lb.NewGroupBackend("group_name", nil)
	bgm := gw.GetBackendGroupManager(host)
	bgm.AddBackendGroup(group)
	// 1. Support to add the backend into the group before adding the group backend into the route forwarder.
	backend3, _ := backend.NewHTTPBackend("127.0.0.1:8003", nil)
	group.AddBackend(backend3)
	// 2. Add the backend group into the route forwarder.
	forwarder.AddBackend(group)
	// 3. Support to add the backend into the group after adding the group backend into the route forwarder.
	backend4, _ := backend.NewHTTPBackend("127.0.0.1:8004", nil)
	group.AddBackend(backend4)

	// Start HTTP server.
	gw.Router().Start("127.0.0.1:80").Wait()

	// TEST THE GATEWAY:
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com'
	// HTTP/1.1 401 Unauthorized
	// Date: Wed, 06 Jan 2021 21:21:25 GMT
	// Content-Length: 0
	//
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Wed, 06 Jan 2021 21:21:56 GMT
	// Server: MTI3LjAuMC4x
	//
	// {"backend":"127.0.0.1:8002"}
	//
	//
	// $ curl -i http://127.0.0.1:80/v1/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 200 OK
	// Content-Length: 29
	// Content-Type: application/json
	// Date: Wed, 06 Jan 2021 21:21:59 GMT
	// Server: MTI3LjAuMC4x
	//
	// {"backend":"127.0.0.1:8001"}
	//
	//
	// $ curl -i http://127.0.0.1:80/v2/test -H 'Host: www.example.com' -H 'Authorization: token authentication_token'
	// HTTP/1.1 404 Not Found
	// Content-Type: text/plain; charset=UTF-8
	// Date: Wed, 06 Jan 2021 21:22:03 GMT
	// Content-Length: 57
	//
	// no route: host=www.example.com, method=GET, path=/v2/test
	//
}
```
