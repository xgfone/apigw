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
	"sync/atomic"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/ship/v3"
)

// NewQPSBackend returns a new QPS backend.
//
// If qps is equal to 0, no limit.
func NewQPSBackend(qps int, next lb.Backend) lb.Backend {
	if qps < 0 {
		qps = 0
	}

	return &qpsBackend{Backend: next, maxQPS: int64(qps)}
}

type qpsBackend struct {
	lb.Backend
	maxQPS int64
	curQPS int64
}

func (b *qpsBackend) UnwrapEndpoint() lb.Backend { return b.Backend }
func (b *qpsBackend) MetaData() map[string]interface{} {
	md := b.Backend.MetaData()
	md["qps"] = b.maxQPS
	return md
}

func (b *qpsBackend) RoundTrip(c context.Context, r lb.Request) (interface{}, error) {
	if b.maxQPS == 0 {
		return b.Backend.RoundTrip(c, r)
	}

	qps := atomic.AddInt64(&b.curQPS, 1)
	defer atomic.AddInt64(&b.curQPS, -1)

	if qps > b.maxQPS {
		return nil, ship.ErrTooManyRequests
	}
	return b.Backend.RoundTrip(c, r)
}
