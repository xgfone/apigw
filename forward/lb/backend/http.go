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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// NewDefaultHTTPClient creates a new http.Client with the default configuration.
func NewDefaultHTTPClient(maxConn int, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     nil,
			TLSHandshakeTimeout: time.Second * 5,

			MaxIdleConns:        maxConn / 10,
			MaxIdleConnsPerHost: maxConn / 100,
			MaxConnsPerHost:     maxConn,

			IdleConnTimeout:       timeout,
			ResponseHeaderTimeout: timeout,
			ExpectContinueTimeout: timeout,
		},
	}
}

// NewHTTPBackend returns a new HTTP backend.
//
// If method is empty, it will use the method of the source request.
//
// It supports the paramether for the path part in backendURL,
// such as "/path/:param1/to/:param2/somewhere".
func NewHTTPBackend(method, backendURL string, client *http.Client) (lb.Backend, error) {
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

	upaths := strings.Split(u.Path, "/")
	paths := make([]string, 0, len(upaths))
	for i, path := range upaths {
		if i == 0 || path != "" {
			paths = append(paths, path)
		}
	}

	host := base64.StdEncoding.EncodeToString([]byte(u.Hostname()))
	return httpBackend{
		u:     u,
		paths: paths,

		url:    u.String(),
		host:   host,
		method: method,
		client: client,
	}, nil
}

type httpBackend struct {
	u     *url.URL
	paths []string

	url    string
	host   string
	method string
	client *http.Client
}

func (e httpBackend) getBackendURL(ctx *ship.Context) (string, error) {
	if e.u == nil {
		return e.url, nil
	}

	_len := len(e.paths)
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

func (e httpBackend) Metadata() map[string]interface{} {
	return map[string]interface{}{"method": e.method, "url": e.url}
}

func (e httpBackend) String() string {
	return fmt.Sprintf("Backend(%s)", e.url)
}

func (e httpBackend) IsHealthy(c context.Context) bool {
	// TODO(xgfone): Check whether the backend node is healthy.
	return true
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
