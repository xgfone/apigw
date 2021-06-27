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

	lock sync.RWMutex
	bgms map[string]*BackendGroupManager
}

// NewGateway returns the new Gateway.
func NewGateway() *Gateway {
	g := &Gateway{
		Gateway: apigw.NewGateway(),
		bgms:    make(map[string]*BackendGroupManager),
	}
	g.Gateway.Context = g
	return g
}

// GetBackendGroupManager returns the backend group manager by the host as the name.
//
// If the backend group manager does not exist, return nil.
func (g *Gateway) GetBackendGroupManager(host string) *BackendGroupManager {
	g.lock.RLock()
	m := g.bgms[host]
	g.lock.RUnlock()
	return m
}

// GetBackendGroupManagers returns all the backend group manager.
func (g *Gateway) GetBackendGroupManagers() (ms []*BackendGroupManager) {
	g.lock.RLock()
	ms = make([]*BackendGroupManager, 0, len(g.bgms))
	for _, m := range g.bgms {
		ms = append(ms, m)
	}
	g.lock.RUnlock()
	return
}

// AddHost replaces the method g.Gateway.AddHost to also add the backend group
// manager.
func (g *Gateway) AddHost(host string) (err error) {
	if err = g.Gateway.AddHost(host); err == nil {
		g.lock.Lock()
		if _, ok := g.bgms[host]; !ok {
			g.bgms[host] = NewBackendGroupManager(host)
		}
		g.lock.Unlock()
	}
	return
}

// DelHost replaces the method g.Gateway.DelHost to also delete the backend group
// manager.
func (g *Gateway) DelHost(host string) (err error) {
	if err = g.Gateway.DelHost(host); err == nil {
		g.lock.Lock()
		defer g.lock.Unlock()
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

/////////////////////////////////////////////////////////////////////////////

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

// GetRoutePolicySelector is a convenient function to get the policy selector
// to forward the request.
func (g *Gateway) GetRoutePolicySelector(host, path, method string) (Selector, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetSelector(), nil
	}
	return nil, err
}

// SetRoutePolicySelector is a convenient function to set the policy selector
// to forward the request.
func (g *Gateway) SetRoutePolicySelector(host, path, method string, s Selector) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetSelector(s)
	}
	return err
}

// SetRoutePolicySelectorByName is equal to SetRoutePolicySelector,
// but get the session from the global registered selectors by the name.
func (g *Gateway) SetRoutePolicySelectorByName(host, path, method, selectorName string) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetSelectorByName(selectorName)
	}
	return err
}

// GetRouteMaxTimeout is a convenient function to get the maximum timeout
// to forward the request.
func (g *Gateway) GetRouteMaxTimeout(host, path, method string) (time.Duration, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetMaxTimeout(), nil
	}
	return 0, err
}

// SetRouteMaxTimeout is a convenient function to set the maximum timeout
// to forward the request.
func (g *Gateway) SetRouteMaxTimeout(host, path, method string, timeout time.Duration) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.SetMaxTimeout(timeout)
	}
	return err
}

// GetRouteBackends is a convenient function to get all the backends
// of the route forwarder.
func (g *Gateway) GetRouteBackends(host, path, method string) ([]BackendInfo, error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		return forwarder.GetBackends(), nil
	}
	return nil, err
}

// AddRouteBackendGroup is a convenient function to add the backend group
// into the route forwarder.
func (g *Gateway) AddRouteBackendGroup(host, path, method string, group BackendGroup) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.AddBackendGroup(group)
	}
	return err
}

// AddRouteBackendWithChecker is a convenient function to add the backend
// with the checker into the route forwarder.
func (g *Gateway) AddRouteBackendWithChecker(host, path, method string,
	backend Backend, checker BackendChecker, duration BackendCheckerDuration) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.AddBackendWithChecker(backend, checker, duration)
	}
	return err
}

// DelRouteBackendByID is a convenient function to delete the backend by the id
// from the route forwarder.
func (g *Gateway) DelRouteBackendByID(host, path, method string, backendID string) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.DelBackendByID(backendID)
	}
	return err
}
