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

package lb_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/apigw/forward/lb/backend"
	"github.com/xgfone/ship/v3"
)

func BenchmarkLBForwarder(b *testing.B) {
	forwarder := lb.NewForwarder("", nil)
	forwarder.AddBackend(backend.NewNoopBackend("server1", true))
	forwarder.AddBackend(backend.NewNoopBackend("server2", true))

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	ctx := ship.NewContext(0, 0)
	ctx.SetReqRes(req, httptest.NewRecorder())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		forwarder.Forward(ctx)
	}
}
