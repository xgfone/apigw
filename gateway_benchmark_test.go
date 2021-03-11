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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xgfone/apigw"
	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/go-service/loadbalancer"
)

type fakeBackend struct {
	name string
}

func newFakeBackend(name string) fakeBackend           { return fakeBackend{name: name} }
func (b fakeBackend) Type() string                     { return "fake" }
func (b fakeBackend) String() string                   { return b.name }
func (b fakeBackend) HealthCheck() lb.HealthCheck      { return lb.HealthCheck{} }
func (b fakeBackend) UserData() interface{}            { return nil }
func (b fakeBackend) MetaData() map[string]interface{} { return nil }
func (b fakeBackend) IsHealthy(context.Context) bool   { return true }
func (b fakeBackend) RoundTrip(c context.Context, r loadbalancer.Request) (loadbalancer.Response, error) {
	// r.(lb.Request).Context().Text(200, b.name)
	return nil, nil
}

func newPanicErrorPlugin(config interface{}) (apigw.Middleware, error) {
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) error {
			if err := next(ctx); err != nil {
				panic(err)
			}
			return nil
		}
	}, nil
}

func newReqCountPlugin(config interface{}) (apigw.Middleware, error) {
	return func(next apigw.Handler) apigw.Handler {
		var count int64
		return func(ctx *apigw.Context) error {
			count++
			defer func() { count-- }()
			return next(ctx)
		}
	}, nil
}

func BenchmarkGatewayWithoutPlugins(b *testing.B) {
	gw := apigw.NewGateway()

	forwarder := lb.NewForwarder("benchmark", nil)
	forwarder.AddBackend(newFakeBackend("backend1"))
	forwarder.AddBackend(newFakeBackend("backend2"))
	gw.RegisterRoute(apigw.Route{
		Host:      "www.example.com",
		Path:      "/v1/:path",
		Method:    http.MethodGet,
		Forwarder: forwarder,
	})

	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com/v1/test", nil)
	resp := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gw.ServeHTTP(resp, req)
	}
}

func BenchmarkGatewayWithPlugins(b *testing.B) {
	gw := apigw.NewGateway()
	gw.RegisterPlugin(apigw.NewPlugin("panic", 2, newPanicErrorPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("count", 1, newReqCountPlugin))

	forwarder := lb.NewForwarder("benchmark", nil)
	forwarder.AddBackend(newFakeBackend("backend1"))
	forwarder.AddBackend(newFakeBackend("backend2"))
	gw.RegisterRoute(apigw.Route{
		Host:      "www.example.com",
		Path:      "/v1/:path",
		Method:    http.MethodGet,
		Forwarder: forwarder,
		Plugins: []apigw.RoutePlugin{
			{Name: "count"},
			{Name: "panic"},
		},
	})

	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com/v1/test", nil)
	resp := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gw.ServeHTTP(resp, req)
	}
}
