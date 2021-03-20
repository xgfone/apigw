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
	"context"
	"testing"
	"time"

	"github.com/xgfone/apigw/forward/lb"
	slb "github.com/xgfone/go-service/loadbalancer"
)

type noopBackend struct {
	name    string
	healthy bool
}

func newNoopBackend(name string, healthy bool) noopBackend {
	return noopBackend{name: name, healthy: healthy}
}

func (b noopBackend) Type() string                                                { return "noop" }
func (b noopBackend) String() string                                              { return b.name }
func (b noopBackend) State() (s slb.EndpointState)                                { return }
func (b noopBackend) MetaData() map[string]interface{}                            { return nil }
func (b noopBackend) IsHealthy(context.Context) bool                              { return b.healthy }
func (b noopBackend) RoundTrip(context.Context, slb.Request) (interface{}, error) { return nil, nil }

func TestForwarder_Backends(t *testing.T) {
	hc := slb.NewHealthCheck()
	defer hc.Stop()

	f := lb.NewForwarder("")
	f.HealthCheck = hc
	backend1 := newNoopBackend("backend1", false)
	backend2 := newNoopBackend("backend2", false)
	backend3 := newNoopBackend("backend3", false)

	hc.SetDefaultOption(slb.HealthCheckOption{Interval: 10 * time.Millisecond})
	hc.Subscribe("", f)
	f.AddBackend(backend1)
	f.AddBackend(backend2)
	f.AddBackend(backend3)
	time.Sleep(time.Millisecond * 200)
	for _, b := range f.GetBackends() {
		if healthy := b.IsHealthy(context.Background()); healthy {
			t.Errorf("Backend %s: expect '%v', but got '%v'", b.String(), false, true)
		}
	}

	f.DelBackend(backend1)
	f.DelBackend(backend2)
	f.DelBackend(backend3)
	time.Sleep(time.Millisecond * 100)
	if backends := f.GetBackends(); len(backends) != 0 {
		t.Error(backends)
	}

	backend1.healthy = true
	backend2.healthy = true
	backend3.healthy = true
	f.AddBackend(backend1)
	f.AddBackend(backend2)
	f.AddBackend(backend3)
	time.Sleep(time.Millisecond * 200)
	for _, b := range f.GetBackends() {
		if healthy := b.IsHealthy(context.Background()); !healthy {
			t.Errorf("Backend %s: expect '%v', but got '%v'", b.String(), true, false)
		}
	}
}
