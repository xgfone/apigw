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
	"net/http"
	"time"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/go-service/loadbalancer"
	"github.com/xgfone/ship/v3"
)

// HTTPBackendConfig is used to configure the http backend.
type HTTPBackendConfig = loadbalancer.HTTPEndpointConfig

// NewHTTPBackend returns a new HTTP backend.
func NewHTTPBackend(addr string, conf *HTTPBackendConfig) (lb.Backend, error) {
	if conf == nil {
		conf = &HTTPBackendConfig{Handler: handleHTTP}
	} else if conf.Handler == nil {
		conf.Handler = handleHTTP
	}
	return loadbalancer.NewHTTPEndpoint(addr, conf)
}

func handleHTTP(r lb.Request, c *http.Client, hr *http.Request) (_ *http.Response, e error) {
	ctx := r.(lb.HTTPRequest).Context()

	start := time.Now()
	if resp, err := c.Do(hr); err != nil {
		e = ship.ErrBadGateway.New(err)
	} else {
		defer resp.Body.Close()

		respHeader := ctx.RespHeader()
		for k, vs := range resp.Header {
			respHeader[k] = vs
		}

		e = ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
	}

	ctx.Logger().Debugf("Forwarding HTTP Request '%s %s' -> '%s %s', cost '%s'",
		ctx.Method(), ctx.RequestURI(), hr.Method, hr.URL.String(), time.Since(start))

	return
}
