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

package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

func init() {
	tp := http.DefaultTransport.(*http.Transport)
	tp.IdleConnTimeout = time.Second * 30
	tp.MaxIdleConnsPerHost = 100
	tp.MaxIdleConns = 0
}

// HTTPBackendConfig is used to configure the http backend.
type HTTPBackendConfig struct {
	Client        *http.Client
	UserData      interface{}
	HealthCheck   lb.HealthCheck
	HealthChecker loadbalancer.HealthChecker
}

// NewHTTPBackend returns a new HTTP backend.
//
// If method is empty, it will use the method of the source request.
//
// It supports the paramether for the path part in backendURL,
// such as "/path/:param1/to/:param2/somewhere".
func NewHTTPBackend(method, backendURL string, config *HTTPBackendConfig) (lb.Backend, error) {
	switch method = strings.ToUpper(method); method {
	case "":
	case http.MethodGet:
	case http.MethodHead:
	case http.MethodPost:
	case http.MethodPut:
	case http.MethodPatch:
	case http.MethodDelete:
	case http.MethodConnect:
	case http.MethodOptions:
	case http.MethodTrace:
	default:
		return nil, fmt.Errorf("invalid http method '%s'", method)
	}

	u, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

	addr := u.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		if !strings.HasPrefix(err.Error(), "missing port") {
			return nil, err
		}

		switch u.Scheme {
		case "http":
			addr = net.JoinHostPort(addr, "80")
		case "https":
			addr = net.JoinHostPort(addr, "443")
		default:
			return nil, fmt.Errorf("unknown http scheme '%s'", u.Scheme)
		}
	}

	if u.Path == "" {
		u.Path = "/"
	}

	var arg bool
	upaths := strings.Split(u.Path, "/")
	paths := make([]string, 0, len(upaths))
	for i, path := range upaths {
		if i == 0 && path == "" {
			paths = append(paths, path)
		} else if path != "" {
			paths = append(paths, path)
			if path[0] == ':' {
				arg = true
			}
		}
	}
	if !arg {
		paths = nil
	}

	var conf HTTPBackendConfig
	if config != nil {
		conf = *config
	}

	host := base64.StdEncoding.EncodeToString([]byte(u.Hostname()))
	return httpBackend{
		u:     u,
		paths: paths,

		url:      u.String(),
		addr:     addr,
		host:     host,
		method:   method,
		client:   conf.Client,
		check:    conf.HealthChecker,
		userdata: conf.UserData,
		hc:       conf.HealthCheck,
	}, nil
}

type httpBackend struct {
	u     *url.URL
	paths []string

	url      string
	addr     string
	host     string
	method   string
	client   *http.Client
	check    loadbalancer.HealthChecker
	userdata interface{}
	hc       lb.HealthCheck
}

func (e httpBackend) getBackendURL(ctx *ship.Context) (string, error) {
	_len := len(e.paths)
	if _len == 0 {
		return e.url, nil
	}

	paths := make([]string, _len)
	for i := 0; i < _len; i++ {
		value := e.paths[i]
		if value != "" && value[0] == ':' {
			value = value[1:]
			if v := ctx.URLParam(value); v != "" {
				value = v
			} else if v = ctx.QueryParam(value); v != "" {
				value = v
			} else if v = ctx.GetHeader(value); v != "" {
				value = v
			} else {
				return "", fmt.Errorf("no value for path param named '%s'", value)
			}
		}
		paths[i] = value
	}

	u := *e.u
	u.Path = strings.Join(paths, "/")
	return u.String(), nil
}

func (e httpBackend) Type() string                { return "http" }
func (e httpBackend) String() string              { return e.url }
func (e httpBackend) HealthCheck() lb.HealthCheck { return e.hc }
func (e httpBackend) UserData() interface{}       { return e.userdata }
func (e httpBackend) MetaData() map[string]interface{} {
	return map[string]interface{}{"method": e.method, "url": e.url}
}

func (e httpBackend) IsHealthy(c context.Context) bool {
	if e.check != nil {
		return e.check(c, e.url) == nil
	}

	dialer := net.Dialer{Timeout: time.Second}
	if conn, err := dialer.DialContext(c, "tcp", e.addr); err == nil {
		conn.Close()
		return true
	}
	return false
}

func (e httpBackend) RoundTrip(c context.Context, r loadbalancer.Request) (loadbalancer.Response, error) {
	ctx := r.(lb.Request).Context()

	url, err := e.getBackendURL(ctx)
	if err != nil {
		return nil, ship.ErrBadRequest.New(err)
	}

	method := e.method
	if method == "" {
		method = ctx.Method()
	}

	nreq, err := ship.NewRequestWithContext(c, method, url, ctx.Body())
	if err != nil {
		return nil, ship.ErrBadGateway.New(err)
	}

	nreq.Header = ctx.Request().Header
	if nreq.Header.Get(ship.HeaderXRealIP) == "" && nreq.Header.Get(ship.HeaderXForwardedFor) == "" {
		nreq.Header.Set(ship.HeaderXRealIP, ctx.RealIP())
	}

	ctx.Logger().Debugf("Forwarding HTTP Request '%s %s' -> '%s %s'",
		ctx.Method(), ctx.RequestURI(), method, url)

	var resp *http.Response
	if e.client == nil {
		resp, err = http.DefaultClient.Do(nreq)
	} else {
		resp, err = e.client.Do(nreq)
	}
	if err != nil {
		return nil, ship.ErrBadGateway.New(err)
	}
	defer resp.Body.Close()

	respHeader := ctx.RespHeader()
	for k, v := range resp.Header {
		respHeader[k] = v
	}

	ctx.SetHeader(ship.HeaderServer, e.host)
	err = ctx.Stream(resp.StatusCode, resp.Header.Get(ship.HeaderContentType), resp.Body)
	return nil, err
}
