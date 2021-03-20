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
	"net/http"

	"github.com/xgfone/apigw"
)

// HTTPRequest is used to represent the request context passed to the backend.
type HTTPRequest interface {
	Context() *apigw.Context
	Request() *http.Request
	Request
}

// NewRequestWithXSessionId is equal to NewRequestWithHeader(c, "X-Session-Id").
func NewRequestWithXSessionId(c *apigw.Context) HTTPRequest {
	return simpleRequest{c}
}

type simpleRequest struct{ ctx *apigw.Context }

func (r simpleRequest) Context() *apigw.Context  { return r.ctx }
func (r simpleRequest) Request() *http.Request   { return r.ctx.Request() }
func (r simpleRequest) RemoteAddrString() string { return r.ctx.RemoteAddr() }
func (r simpleRequest) SessionID() string {
	vs := r.ctx.ReqHeader()["X-Session-Id"]
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// NewRequestWithSessionID returns a http request with the session id,
// which may be empty.
func NewRequestWithSessionID(c *apigw.Context, sessionID string) HTTPRequest {
	return sidRequest{ctx: c, sid: sessionID}
}

type sidRequest struct {
	ctx *apigw.Context
	sid string
}

func (r sidRequest) SessionID() string        { return r.sid }
func (r sidRequest) RemoteAddrString() string { return r.ctx.RemoteAddr() }
func (r sidRequest) Request() *http.Request   { return r.ctx.Request() }
func (r sidRequest) Context() *apigw.Context  { return r.ctx }

// NewRequestWithHeader returns a http request with the header,
// which gets the session id from the given header.
func NewRequestWithHeader(c *apigw.Context, headerKey string) HTTPRequest {
	return headerRequest{ctx: c, header: headerKey}
}

type headerRequest struct {
	ctx    *apigw.Context
	header string
}

func (r headerRequest) SessionID() string        { return r.ctx.ReqHeader().Get(r.header) }
func (r headerRequest) RemoteAddrString() string { return r.ctx.RemoteAddr() }
func (r headerRequest) Request() *http.Request   { return r.ctx.Request() }
func (r headerRequest) Context() *apigw.Context  { return r.ctx }
