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
	"fmt"

	"github.com/xgfone/apigw/forward/lb"
	"github.com/xgfone/go-service/loadbalancer"
)

var _ lb.Backend = noopBackend{}

// NewNoopBackend returns a new noop backend, which is only used to test
// the performance of the gateway framework.
func NewNoopBackend(name string, noresp ...bool) lb.Backend {
	var notresp bool
	if len(noresp) > 0 {
		notresp = noresp[0]
	}
	return noopBackend{name: name, notresp: notresp}
}

type noopBackend struct {
	name    string
	notresp bool
}

func (b noopBackend) Metadata() map[string]interface{} {
	return map[string]interface{}{"name": b.name, "noresp": b.notresp}
}

func (b noopBackend) String() string {
	return fmt.Sprintf("Backend(%s)", b.name)
}

func (b noopBackend) IsHealthy(c context.Context) bool {
	return true
}

func (b noopBackend) RoundTrip(c context.Context, r loadbalancer.Request) (
	loadbalancer.Response, error) {
	if b.notresp {
		return nil, nil
	}
	return nil, r.(lb.Request).Context().Text(200, b.name)
}
