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

package lb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/xgfone/apigw"
)

type fakeBackend struct{ name string }

func newFakeBackend(name string) fakeBackend { return fakeBackend{name: name} }

func (b fakeBackend) ID() string                                              { return b.name }
func (b fakeBackend) Type() string                                            { return "fake" }
func (b fakeBackend) State() (s BackendState)                                 { return }
func (b fakeBackend) MetaData() map[string]interface{}                        { return nil }
func (b fakeBackend) RoundTrip(context.Context, Request) (interface{}, error) { return nil, nil }

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
			atomic.AddInt64(&count, 1)
			defer atomic.AddInt64(&count, -1)
			return next(ctx)
		}
	}, nil
}

func BenchmarkGatewayWithoutPlugins(b *testing.B) {
	forwarder := NewForwarder("benchmark")
	forwarder.Session = nil
	forwarder.AddBackendWithChecker(newFakeBackend("backend1"), nil, BackendCheckerDurationZero)
	forwarder.AddBackendWithChecker(newFakeBackend("backend2"), nil, BackendCheckerDurationZero)

	gw := NewGateway()
	gw.AddHost("www.example.com")
	gw.RegisterRoute(apigw.Route{
		Matcher:   apigw.NewMatcher("www.example.com", "/v1/test", http.MethodGet),
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
	forwarder := NewForwarder("benchmark")
	forwarder.AddBackendWithChecker(newFakeBackend("backend1"), nil, BackendCheckerDurationZero)
	forwarder.AddBackendWithChecker(newFakeBackend("backend2"), nil, BackendCheckerDurationZero)

	gw := apigw.NewGateway()
	gw.AddHost("www.example.com")
	gw.RegisterPlugin(apigw.NewPlugin("panic", 2, newPanicErrorPlugin))
	gw.RegisterPlugin(apigw.NewPlugin("count", 1, newReqCountPlugin))
	_, err := gw.RegisterRoute(apigw.Route{
		Matcher:   apigw.NewMatcher("www.example.com", "/v1/test", http.MethodGet),
		Forwarder: forwarder,
		Plugins: []apigw.RoutePlugin{
			{Name: "count"},
			{Name: "panic"},
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com/v1/test", nil)
	resp := httptest.NewRecorder()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gw.ServeHTTP(resp, req)
	}
}
