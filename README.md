# apigw [![Build Status](https://travis-ci.org/xgfone/apigw.svg?branch=master)](https://travis-ci.org/xgfone/apigw) [![GoDoc](https://godoc.org/github.com/xgfone/apigw?status.svg)](https://pkg.go.dev/github.com/xgfone/apigw) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/apigw/master/LICENSE)

Another simple, flexible, high performance api gateway library implemented by Go. For the binary program, see the another repository [apigateway](https://github.com/xgfone/apigateway).


### Features
- High performance, flexible.
    ```shell
    # Environment: 8C8GB, NIC 1000Mb/s, CentOS 7.4.1708 64bit, Go1.16
    $ wrk -t 8 -c 1000 -d 30s --latency -H 'Host: www.exampletest.com' http://192.168.1.10/v1/test
    Running 30s test @ http://192.168.1.10/v1/test
      8 threads and 1000 connections
      Thread Stats   Avg      Stdev     Max   +/- Stdev
        Latency    39.04ms   21.66ms 331.90ms   73.64%
        Req/Sec     3.17k   428.99     8.31k    84.21%
      Latency Distribution
         50%   35.77ms
         75%   50.02ms
         90%   66.15ms
         99%  103.17ms
      756985 requests in 30.01s, 109.73MB read
      Socket errors: connect 0, read 0, write 231, timeout 0
    Requests/sec:  25223.57
    Transfer/sec:  3.66MB
    ````
- Too few core engine codes, ~500 lines.
    ```shell
    $ cloc --exclude-dir=plugins --not-match-f=_test.go --include-lang=Go --quiet .
    -------------------------------------------------------------------------------
    Language                     files          blank        comment           code
    -------------------------------------------------------------------------------
    Go                              11            140            227            522
    -------------------------------------------------------------------------------
    SUM:                            11            140            227            522
    -------------------------------------------------------------------------------
    ```
- Most of the functions are implemented by the plugin mode.


### Difference between Middleware and Plugin
- Plugin is pluggable during running, and run after routing the request.
- Middleware is unpluggable after running, and run before routing the request.

**Notice:** The framework is based on [ship](https://github.com/xgfone/ship), so it's based on `Path` and `Method` of the request URL to route the request at first, then the api gateway framework takes over the handling and forwards it to one of the backends, such as routing based on the header or rewriting the request.


### TODO List
- [ ] Add some authentications.
- [ ] Add some built-in plugins and middlewares.
- [ ] Add the health check for the backend, that's upstream server.
- [ ] Add the support of a group of the upstream servers as the backend.
- [x] Optimize the HTTP backend forwarder.
- [ ] Others.


## Install
```shell
$ go get -u github.com/xgfone/apigw
```


## Example
```go
package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/xgfone/apigw"
	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/apigw/forward/lb/backend"
	"github.com/xgfone/apigw/plugin"
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

func newTokenPlugin(config interface{}) (apigw.Middleware, error) {
	token := config.(string)
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) (err error) {
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
	token := "authentication_token"

	gw := apigw.NewGateway()
	gw.RegisterMiddlewares(gwMiddleware("middleware1"), gwMiddleware("middleware2"))
	gw.RegisterPlugin(plugin.NewPlugin("panic", 3, newPanicPlugin))
	gw.RegisterPlugin(plugin.NewPlugin("token", 1, newTokenPlugin))
	gw.RegisterPlugin(plugin.NewPlugin("log", 2, newLogPlugin))

	// Register the route and its backends.
	backend1, _ := backend.NewHTTPBackend("", "http://127.0.0.1:8001/:path", nil)
	backend2, _ := backend.NewHTTPBackend("", "http://127.0.0.1:8002/:path", nil)

	forwarder := lb.NewForwarder(time.Minute)
	forwarder.EndpointManager().AddEndpoint(backend1)
	forwarder.EndpointManager().AddEndpoint(backend2)
	gw.RegisterRoute(apigw.Route{
		Host:      "www.example.com",
		Path:      "/v1/:path",
		Method:    http.MethodGet,
		Forwarder: forwarder,
		PluginConfigs: []apigw.RoutePluginConfig{
			{PluginName: "token", PluginConfig: token},
			{PluginName: "log"},

			/// We don't configure the panic plugin for the current route,
			/// so it won't be used when triggering the route.
			// {PluginName: "panic"},
		},
	})

	// Start HTTP server.
	gw.Router().Start("127.0.0.1:12345").Wait()
}
```
