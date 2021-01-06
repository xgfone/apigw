# apigw [![Build Status](https://travis-ci.org/xgfone/apigw.svg?branch=master)](https://travis-ci.org/xgfone/apigw) [![GoDoc](https://godoc.org/github.com/xgfone/apigw?status.svg)](http://godoc.org/github.com/xgfone/apigw) [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://raw.githubusercontent.com/xgfone/apigw/master/LICENSE)

Another simple, flexible, high performance api gateway library implemented by Go.


### Features
- High performance, flexible.
- Too few core engine codes, about 400~500 lines.
    ```shell
    $ cloc --exclude-dir=cmd --exclude-dir=plugins --include-lang=Go --quiet .
    github.com/AlDanial/cloc v 1.88  T=0.03 s (288.3 files/s, 26918.9 lines/s)
    -------------------------------------------------------------------------------
    Language                     files          blank        comment           code
    -------------------------------------------------------------------------------
    Go                               8            105            186            456
    -------------------------------------------------------------------------------
    SUM:                             8            105            186            456
    -------------------------------------------------------------------------------
    ```
- Most of the functions are implemented by the plugin mode.


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
	"github.com/xgfone/apigw/forward"
	"github.com/xgfone/apigw/forward/endpoint"
	"github.com/xgfone/apigw/plugin"
	"github.com/xgfone/ship/v3"
)

func gwMiddleware(name string) apigw.Middleware {
	return func(next ship.Handler) ship.Handler {
		return func(ctx *ship.Context) error {
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
	gw.RegisterRoute(apigw.Route{
		Host:      "www.example.com",
		Name:      "test",
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
