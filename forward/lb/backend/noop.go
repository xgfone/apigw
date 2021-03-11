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
)

var _ lb.Backend = noopBackend{}

func init() {
	RegisterBuilder(NewBuilder("noop", func(c BuilderContext) (lb.Backend, error) {
		name, _ := c.MetaData["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("missing the string name")
		}
		return NewNoopBackend(name, c.UserData), nil
	}))
}

// NewNoopBackend returns a new noop backend, which is only used to test
// the performance of the gateway framework.
func NewNoopBackend(name string, userdata interface{}) lb.Backend {
	return noopBackend{name: name, data: userdata}
}

type noopBackend struct {
	name string
	data interface{}
}

func (b noopBackend) Type() string                     { return "noop" }
func (b noopBackend) String() string                   { return b.name }
func (b noopBackend) IsHealthy(c context.Context) bool { return true }
func (b noopBackend) HealthCheck() lb.HealthCheck      { return lb.HealthCheck{} }
func (b noopBackend) UserData() interface{}            { return b.data }
func (b noopBackend) MetaData() map[string]interface{} {
	return map[string]interface{}{"name": b.name}
}
func (b noopBackend) RoundTrip(c context.Context, r lb.Request) (lb.Response, error) {
	return nil, r.(lb.HTTPRequest).Context().Text(200, b.name)
}
