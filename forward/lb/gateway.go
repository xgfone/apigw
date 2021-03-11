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
	return &Gateway{
		Gateway: apigw.NewGateway(),
		bgms:    make(map[string]*BackendGroupManager),
	}
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

// GetRouteForwarder returns the forwarder of the route.
func (g *Gateway) GetRouteForwarder(host, path, method string) (Forwarder, error) {
	route, err := g.GetRoute(host, path, method)
	if err != nil {
		return nil, err
	}
	return route.Forwarder.(Forwarder), nil
}

// GetRouteBackends returns all the backends of the route, which not containing
// the group backend. For the group backend, you need to call the method
// g.GetBackendGroupManager(host) to get them.
func (g *Gateway) GetRouteBackends(host, path, method string) (bs []Backend, err error) {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		bs = forwarder.GetBackends()
	}
	return
}

// AddRouteBackends adds the backends into the route.
func (g *Gateway) AddRouteBackends(host, path, method string, backends ...Backend) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.AddBackends(backends...)
	}
	return nil
}

// DelRouteBackends deletes the backends into the route.
func (g *Gateway) DelRouteBackends(host, path, method string, backends ...Backend) error {
	forwarder, err := g.GetRouteForwarder(host, path, method)
	if err == nil {
		forwarder.DelBackends(backends...)
	}
	return nil
}

// GetBackendGroupManager returns the backend group manager by the host
// as the name.
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
