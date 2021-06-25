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

package apigw

import "sync"

// DefaultGatewayManager is the default global gateway manager.
var DefaultGatewayManager = NewGatewayManager()

// GatewayManager is used to manage a group of gateways.
type GatewayManager struct {
	lock     sync.RWMutex
	gateways map[string]*Gateway
}

// NewGatewayManager returns a new gateway manager.
func NewGatewayManager() *GatewayManager {
	return &GatewayManager{gateways: make(map[string]*Gateway)}
}

// RegisterGateway registers the gateway and returns true if successfully,
// or returns false if the gateway has been registered.
//
// Notice: you can use this to manage all the global gateways.
func (m *GatewayManager) RegisterGateway(g *Gateway) (ok bool) {
	name := g.Name()
	m.lock.Lock()
	if _, ok = m.gateways[name]; !ok {
		m.gateways[name] = g
	}
	m.lock.Unlock()
	return !ok
}

// UnregisterGateway unregisters the gateway by the name.
//
// If the gateway does not exist, do nothing and return nil.
func (m *GatewayManager) UnregisterGateway(name string) *Gateway {
	m.lock.Lock()
	g, ok := m.gateways[name]
	if ok {
		delete(m.gateways, name)
	}
	m.lock.Unlock()
	return g
}

// GetGateway returns the registered gateway by the name.
//
// Return nil if the gateway does not exist.
func (m *GatewayManager) GetGateway(name string) *Gateway {
	m.lock.RLock()
	g := m.gateways[name]
	m.lock.RUnlock()
	return g
}

// GetGateways returns all the registered gateways.
func (m *GatewayManager) GetGateways() (gateways []*Gateway) {
	m.lock.RLock()
	gateways = make([]*Gateway, 0, len(m.gateways))
	for _, g := range m.gateways {
		gateways = append(gateways, g)
	}
	m.lock.RUnlock()
	return
}
