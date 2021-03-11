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
	"github.com/xgfone/apigw"
	"github.com/xgfone/go-service/loadbalancer"
)

// HTTPRequest is used to represent the request context passed to the backend.
type HTTPRequest interface {
	loadbalancer.Request
	Context() *apigw.Context
}

// NewRequest returns the loadbackend request.
//
// Notice: getSessionID may be nil.
func NewRequest(ctx *apigw.Context, getSessionID func(*apigw.Context) string) HTTPRequest {
	return sidRequest{ctx: ctx, sid: getSessionID}
}

type simpleRequest struct {
	ctx *apigw.Context
}

func (r simpleRequest) Context() *apigw.Context  { return r.ctx }
func (r simpleRequest) RemoteAddrString() string { return r.ctx.RemoteAddr() }

type sidRequest struct {
	ctx *apigw.Context
	sid func(*apigw.Context) string
}

func (r sidRequest) RemoteAddrString() string { return r.ctx.RemoteAddr() }
func (r sidRequest) SessionID() string        { return r.sid(r.ctx) }
func (r sidRequest) Context() *apigw.Context  { return r.ctx }
