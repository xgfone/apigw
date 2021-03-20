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
	"sync"
	"time"

	"github.com/xgfone/apigw"
)

// DefaultGateway is the default global gateway.
var DefaultGateway = NewGateway()

// Gateway is a gateway based on the loadbalancer as the backend forwarder.
type Gateway struct {
	*apigw.Gateway

	bgms  map[string]*BackendGroupManager
	mlock sync.RWMutex
}

// NewGateway returns the new Gateway.
func NewGateway() *Gateway {
	g := &Gateway{Gateway: apigw.NewGateway(), bgms: make(map[string]*BackendGroupManager)}
	g.Gateway.Context = g
	return g
}

// AddHost replaces the method g.Gateway.AddHost to also add the backend group
// manager.
func (g *Gateway) AddHost(host string) (err error) {
	if err = g.Gateway.AddHost(host); err == nil {
		g.mlock.Lock()
		if _, ok := g.bgms[host]; !ok {
			g.bgms[host] = NewBackendGroupManager(host)
		}
		g.mlock.Unlock()
	}
	return
}

// DelHost replaces the method g.Gateway.DelHost to also delete the backend group
// manager.
func (g *Gateway) DelHost(host string) (err error) {
	if err = g.Gateway.DelHost(host); err == nil {
		g.mlock.Lock()
		defer g.mlock.Unlock()
		if m, ok := g.bgms[host]; ok {
			delete(g.bgms, host)
			m.Close()
		}
	}
	return
}

// GetRouteForwarder is a convenient function to get the forwarder of the route.
func (g *Gateway) GetRouteForwarder(host, path, method string) (*Forwarder, error) {
	route, err := g.GetRoute(host, path, method)
	if err != nil {
		return nil, err
	}
	return route.Forwarder.(*Forwarder), nil
}

// GetRouteSessionTimeout is a convenient function to get the session timeout
// of the route.
func (g *Gateway) GetRouteSessionTimeout(host, path, method string) (time.Duration, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetSessionTimeout(), nil
	}
	return 0, err
}

// SetRouteSessionTimeout is a convenient function to set the session timeout
// of the route.
func (g *Gateway) SetRouteSessionTimeout(host, path, method string, timeout time.Duration) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetSessionTimeout(timeout)
	}
	return err
}

// GetRouteSessionManager is a convenient function to get the session manager
// of the route.
func (g *Gateway) GetRouteSessionManager(host, path, method string) (Session, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetSession(), nil
	}
	return nil, err
}

// SetRouteSessionManager is a convenient function to set the session manager
// of the router.
func (g *Gateway) SetRouteSessionManager(host, path, method string, s Session) error {
	if forwarder, err := g.GetRouteForwarder(host, path, method); err != nil {
		return err
	} else if _type := s.Type(); forwarder.GetSession().Type() == _type {
		return nil
	} else if old := forwarder.SetSession(s); old != nil {
		old.Close()
	}
	return nil
}

// GetRoutePolicySelector is a convenient function to get the policy selector
// to forward the route.
func (g *Gateway) GetRoutePolicySelector(host, path, method string) (Selector, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetSelector(), nil
	}
	return nil, err
}

// SetRoutePolicySelector is a convenient function to set the policy selector
// to forward the route.
func (g *Gateway) SetRoutePolicySelector(host, path, method string, s Selector) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetSelector(s)
	}
	return err
}

// SetRoutePolicySelectorByString is equal to SetRoutePolicySelector,
// but get the session from the global registered selectors as the new session.
func (g *Gateway) SetRoutePolicySelectorByString(host, path, method string, selector string) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetSelectorByString(selector)
	}
	return err
}

// GetRouteHealthCheckOption is a convenient function to get the health check
// option of the route.
func (g *Gateway) GetRouteHealthCheckOption(host, path, method string) (HealthCheckOption, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetHealthCheckOption(), nil
	}
	return HealthCheckOption{}, err
}

// SetRouteHealthCheckOption is a convenient function to set the health check
// option of the route.
func (g *Gateway) SetRouteHealthCheckOption(host, path, method string, option HealthCheckOption) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetHealthCheckOption(option)
	}
	return err
}

// GetRouteBackends is a convenient function to get all the backends of the route.
func (g *Gateway) GetRouteBackends(host, path, method string) (bs []Backend, err error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		bs = forwarder.GetBackends()
	}
	return
}

// AddRouteBackends is a convenient function to add the backends into the route.
func (g *Gateway) AddRouteBackends(host, path, method string, backends ...Backend) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.AddBackends(backends...)
	}
	return err
}

// DelRouteBackends is a convenient function to delete the backends from the route.
func (g *Gateway) DelRouteBackends(host, path, method string, backends ...Backend) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.DelBackends(backends...)
	}
	return err
}

// DelRouteBackendsByString is a convenient function to delete the backends
// by the string from the route.
func (g *Gateway) DelRouteBackendsByString(host, path, method string, backends ...string) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.DelBackendsByString(backends...)
	}
	return err
}

// GetBackendGroupManager returns the backend group manager by the host as the name.
//
// If the backend group manager does not exist, return nil.
func (g *Gateway) GetBackendGroupManager(host string) *BackendGroupManager {
	g.mlock.RLock()
	m := g.bgms[host]
	g.mlock.RUnlock()
	return m
}

// GetBackendGroupManagers returns all the backend group manager.
func (g *Gateway) GetBackendGroupManagers() (ms []*BackendGroupManager) {
	g.mlock.RLock()
	ms = make([]*BackendGroupManager, 0, len(g.bgms))
	for _, m := range g.bgms {
		ms = append(ms, m)
	}
	g.mlock.RUnlock()
	return
}
