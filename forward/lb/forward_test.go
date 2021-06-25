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
	"context"
	"fmt"
	"testing"
	"time"

	lb "github.com/xgfone/go-loadbalancer"
)

type noopBackend struct{ name string }

func newNoopBackend(name string) noopBackend           { return noopBackend{name} }
func (b noopBackend) ID() string                       { return b.name }
func (b noopBackend) Type() string                     { return "noop" }
func (b noopBackend) String() string                   { return b.name }
func (b noopBackend) State() (s BackendState)          { return }
func (b noopBackend) MetaData() map[string]interface{} { return nil }
func (b noopBackend) RoundTrip(context.Context, Request) (interface{}, error) {
	return nil, nil
}

func newBackendChecker() BackendChecker {
	return BackendCheckerFunc(func(context.Context) error { return nil })
}

func TestForwarder(t *testing.T) {
	checker := newBackendChecker()
	backend1 := newNoopBackend("backend1")
	backend2 := newNoopBackend("backend2")
	backend3 := newNoopBackend("backend3")
	backend4 := newNoopBackend("backend4")

	g := NewGroupBackend("group")
	g.AddBackendWithChecker(backend1, nil, BackendCheckerDurationZero)
	g.AddBackendWithChecker(backend2, checker, BackendCheckerDurationZero)
	defer g.Close()

	f := NewForwarder("default")
	f.AddBackendWithChecker(g, nil, BackendCheckerDurationZero)
	f.AddBackendWithChecker(backend3, nil, BackendCheckerDurationZero)
	f.AddBackendWithChecker(backend4, checker, BackendCheckerDurationZero)
	defer f.Close()

	time.Sleep(time.Millisecond * 200)
	backends := f.GetBackends()
	if len(backends) != 3 {
		ids := make([]string, len(backends))
		for i, b := range backends {
			ids[i] = b.ID()
		}
		t.Errorf("incomplete backends: %v", ids)
	} else {
		for _, b := range f.GetBackends() {
			switch id := b.Backend.ID(); id {
			case "group", "backend3", "backend4":
			default:
				t.Errorf("unknown backend '%s'", id)
			}
		}
	}

	var eps lb.Endpoints
	f.LoadBalancer.Inspect(func(_eps lb.Endpoints) {
		for _, ep := range _eps {
			fmt.Printf("+++++ %s: %+v\n", ep.ID(), ep)
		}
		eps = append(eps, _eps...)
	})
	if len(eps) != 4 {
		t.Errorf("incomplete endpoints: %v", eps)
	} else {
		for _, ep := range eps {
			switch id := ep.ID(); id {
			case "backend1", "backend2", "backend3", "backend4":
			default:
				t.Errorf("unknown endpoints '%s'", id)
			}
		}
	}

	f.DelBackendByID(backend3.ID())
	g.DelBackendByID(backend1.ID())
	time.Sleep(time.Millisecond * 200)

	backends = f.GetBackends()
	if len(backends) != 2 {
		ids := make([]string, len(backends))
		for i, b := range backends {
			ids[i] = b.ID()
		}
		t.Errorf("incomplete backends: %v", ids)
	} else {
		for _, b := range f.GetBackends() {
			switch id := b.Backend.ID(); id {
			case "group", "backend4":
			default:
				t.Errorf("unknown backend '%s'", id)
			}
		}
	}

	eps = lb.Endpoints{}
	f.LoadBalancer.Inspect(func(_eps lb.Endpoints) { eps = append(eps, _eps...) })
	if len(eps) != 2 {
		t.Errorf("incomplete endpoints: %v", eps)
	} else {
		for _, ep := range eps {
			switch id := ep.ID(); id {
			case "backend2", "backend4":
			default:
				t.Errorf("unknown endpoints '%s'", id)
			}
		}
	}
}
