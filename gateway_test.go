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
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xgfone/apigw"
)

func TestGateway_NotFound(t *testing.T) {
	gw := apigw.NewGateway()
	gw.SetDefaultNotFound(func(c *apigw.Context) error { return c.Text(200, "notfound") })

	// 1. no domain host
	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com", nil)
	gw.ServeHTTP(rec, req)
	if s := rec.Body.String(); s != "notfound" {
		t.Errorf("expect '%s', but got '%s'\n", "notfound", s)
	}

	gw.SetHostNotFound(`[a-zA-z0-9]+\.example\.com`, func(ctx *apigw.Context) (err error) {
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
	_, err := gw.RegisterRoute(apigw.Route{
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
	if s := rec.Body.String(); s != "test" {
		t.Errorf("expect '%s', but got '%s'\n", "test", s)
	}
}

type testForwarder struct{ name string }

func (f testForwarder) Forward(c *apigw.Context) error { return c.Text(200, f.name) }
func (f testForwarder) Name() string                   { return f.name }
func (f testForwarder) Close() error                   { return nil }

func TestGatewayMiddleware(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	host := "www.example.com"
	rehost := `[a-zA-z0-9]+\.example\.com`

	gw := apigw.NewGateway()
	gw.RegisterGlobalMiddlewares(testMiddleware("global", 1, buf), testMiddleware("global", 2, buf))
	gw.RegisterHostMiddlewares(host, testMiddleware(host, 1, buf), testMiddleware(host, 2, buf))
	gw.RegisterHostMiddlewares(rehost, testMiddleware(rehost, 1, buf), testMiddleware(rehost, 2, buf))

	if err := gw.AddHost(rehost); err != nil {
		t.Fatalf("fail to add the host '%s': %s", host, err)
	}

	route := apigw.Route{
		Host:      rehost,
		Path:      "/",
		Method:    http.MethodGet,
		Forwarder: testForwarder{name: "test"},
	}
	if _, err := gw.RegisterRoute(route); err != nil {
		t.Fatalf("fail to register the router '%s': %s", route.Name(), err)
	}

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "http://www.example.com", nil)
	gw.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "test" {
		t.Errorf("code: %d != 200, body: '%s' != 'test'", rec.Code, rec.Body.String())
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	expectedLines := []string{
		"global 1 before",
		"global 2 before",
		"www.example.com 1 before",
		"www.example.com 2 before",
		`[a-zA-z0-9]+\.example\.com 1 before`,
		`[a-zA-z0-9]+\.example\.com 2 before`,
		`[a-zA-z0-9]+\.example\.com 2 after`,
		`[a-zA-z0-9]+\.example\.com 1 after`,
		"www.example.com 2 after",
		"www.example.com 1 after",
		"global 2 after",
		"global 1 after",
	}

	if len1, len2 := len(lines), len(expectedLines); len1 != len2 {
		t.Errorf("expect %d lines, but got %d lines", len2, len1)
		for i, line := range lines {
			t.Errorf("%d line: %s", i, line)
		}
	} else {
		for i := 0; i < len1; i++ {
			if lines[i] != expectedLines[i] {
				t.Errorf("%d line: expect '%s', but got '%s'", i, expectedLines[i], lines[i])
			}
		}
	}
}

func testMiddleware(host string, index int, buf *bytes.Buffer) apigw.Middleware {
	return func(next apigw.Handler) apigw.Handler {
		return func(ctx *apigw.Context) (err error) {
			fmt.Fprintf(buf, "%s %d before\n", host, index)
			err = next(ctx)
			fmt.Fprintf(buf, "%s %d after\n", host, index)
			return
		}
	}
}

func panicError(t *testing.T, name string, err error) {
	if err != nil {
		t.Fatalf("%s: %s", name, err)
	}
}

func TestGateway_SetDefaultHost(t *testing.T) {
	g := apigw.NewGateway()
	panicError(t, "www.example.com", g.AddHost("www.example.com"))
	panicError(t, "www1.example.com", g.AddHost("www1.example.com"))
	panicError(t, "set_default_host", g.SetDefaultHost("www.example.com"))

	if host, _ := g.Router().GetDefaultRouter(); host != "www.example.com" {
		t.Errorf("expect host '%s', but got '%s'", "www.example.com", host)
	}

	_, err := g.RegisterRoute(apigw.Route{
		Host:      "www.example.com",
		Path:      "/",
		Method:    http.MethodGet,
		Forwarder: testForwarder{"route"},
	})
	panicError(t, "register_route1", err)

	_, err = g.RegisterRoute(apigw.Route{
		Host:      "www1.example.com",
		Path:      "/",
		Method:    http.MethodGet,
		Forwarder: testForwarder{"route1"},
	})
	panicError(t, "register_route2", err)

	if routers := g.Router().Routers(); len(routers) != 2 {
		t.Errorf("expect %d hosts ,but got %d", 2, len(routers))
	} else {
		for host, router := range routers {
			switch host {
			case "www.example.com":
				if rs := router.Routes(); len(rs) != 1 {
					t.Errorf("wrong routes: %v", rs)
				}
			case "www1.example.com":
				if rs := router.Routes(); len(rs) != 1 {
					t.Errorf("wrong routes: %v", rs)
				}
			default:
				t.Errorf("unknown host '%s'", host)
			}
		}
	}

	rec := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "http://www1.example.com", nil)
	panicError(t, "new_request1", err)
	if g.ServeHTTP(rec, req); rec.Code != 200 || rec.Body.String() != "route1" {
		t.Errorf("expect: code=%d, body=%s, but got: code=%d, body=%s",
			200, "route1", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req, err = http.NewRequest(http.MethodGet, "http://www2.example.com", nil)
	panicError(t, "new_request2", err)
	if g.ServeHTTP(rec, req); rec.Code != 200 || rec.Body.String() != "route" {
		t.Errorf("expect: code=%d, body=%s, but got: code=%d, body=%s",
			200, "route", rec.Code, rec.Body.String())
	}
}
