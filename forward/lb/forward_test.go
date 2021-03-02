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
	"github.com/xgfone/go-service/loadbalancer"
)

type noopBackend struct {
	name    string
	healthy bool
}

func newNoopBackend(name string, healthy bool) noopBackend {
	return noopBackend{name: name, healthy: healthy}
}

func (b noopBackend) Type() string                     { return "noop" }
func (b noopBackend) String() string                   { return b.name }
func (b noopBackend) UserData() interface{}            { return nil }
func (b noopBackend) MetaData() map[string]interface{} { return nil }
func (b noopBackend) IsHealthy(c context.Context) bool { return b.healthy }
func (b noopBackend) HealthCheck() (hc lb.HealthCheck) { return }
func (b noopBackend) RoundTrip(c context.Context, r loadbalancer.Request) (
	loadbalancer.Response, error) {
	return nil, nil
}
func (b noopBackend) Metadata() map[string]interface{} {
	return map[string]interface{}{"name": b.name}
}

func TestForwarder_Backends(t *testing.T) {
	f := lb.NewForwarder("", 0)
	backend1 := newNoopBackend("backend1", false)
	backend2 := newNoopBackend("backend2", false)
	backend3 := newNoopBackend("backend3", false)

	f.AddBackend(backend1)
	f.AddBackend(backend2)
	f.AddBackend(backend3)
	for _, b := range f.Backends() {
		if healthy := b.IsHealthy(context.Background()); !healthy {
			t.Errorf("Backend %s: expect '%v', but got '%v'", b.String(), true, false)
		}
	}

	f.DelBackend(backend1)
	f.DelBackend(backend2)
	f.DelBackend(backend3)
	if backends := f.Backends(); len(backends) != 0 {
		t.Error(backends)
	}

	f.HealthCheck = loadbalancer.NewHealthCheck()
	f.HealthCheck.Interval = 10 * time.Millisecond
	f.HealthCheck.Subscribe("", f)
	f.AddBackend(backend1)
	f.AddBackend(backend2)
	f.AddBackend(backend3)
	time.Sleep(time.Millisecond * 200)
	for _, b := range f.Backends() {
		if healthy := b.IsHealthy(context.Background()); healthy {
			t.Errorf("Backend %s: expect '%v', but got '%v'", b.String(), false, true)
		}
	}

	f.DelBackend(backend1)
	f.DelBackend(backend2)
	f.DelBackend(backend3)
	time.Sleep(time.Millisecond * 100)
	if backends := f.Backends(); len(backends) != 0 {
		t.Error(backends)
	}

	backend1.healthy = true
	backend2.healthy = true
	backend3.healthy = true
	f.AddBackend(backend1)
	f.AddBackend(backend2)
	f.AddBackend(backend3)
	time.Sleep(time.Millisecond * 200)
	for _, b := range f.Backends() {
		if healthy := b.IsHealthy(context.Background()); !healthy {
			t.Errorf("Backend %s: expect '%v', but got '%v'", b.String(), true, false)
		}
	}

	f.HealthCheck.Stop()

}
