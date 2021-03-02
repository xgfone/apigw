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
	"net/http/httptest"
	"testing"
)

func TestGateway_NotFound(t *testing.T) {
	gw := NewGateway()
	gw.SetDefaultNotFound(func(c *Context) error { return c.Text(200, "notfound") })

	// 1. no domain host
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com", nil)
	gw.ServeHTTP(rec, req)
	if s := rec.Body.String(); s != "notfound" {
		t.Errorf("expect '%s', but got '%s'\n", "notfound", s)
	}

	gw.SetHostNotFound(`[a-zA-z0-9]+\.example\.com`, func(ctx *Context) (err error) {
		return ctx.Text(404, "www.example.com not found")
	})

	// 2. has domain host and no routes
	if err := gw.AddHost(`[a-zA-z0-9]+\.example\.com`); err != nil {
		t.Errorf("fail to add host: %v", err)
	}
	rec = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "http://www.example.com", nil)
	gw.ServeHTTP(rec, req)
	if s := rec.Body.String(); s != "www.example.com not found" {
		t.Errorf("expect '%s', but got '%s'\n", "www.example.com not found", s)
	}

	// 3. has domain host and routes
	_, err := gw.RegisterRoute(Route{
		Host:      `[a-zA-z0-9]+\.example\.com`,
		Path:      "/",
		Method:    http.MethodGet,
		Forwarder: testForwarder{name: "test"},
	})
	if err != nil {
		t.Errorf("fail to register route: %v\n", err)
	}
	rec = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "http://www.example.com", nil)
	gw.ServeHTTP(rec, req)
	if s := rec.Body.String(); s != "OK" {
		t.Errorf("expect '%s', but got '%s'\n", "OK", s)
	}
}

type testForwarder struct {
	name string
}

func (f testForwarder) Forward(c *Context) error { return c.Text(200, "OK") }
func (f testForwarder) Name() string             { return f.name }
func (f testForwarder) Close() error             { return nil }
