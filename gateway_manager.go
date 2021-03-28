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

// GatewayManager is used to manage a group of gateways.
type GatewayManager struct {
	gateways sync.Map
}

// NewGatewayManager returns a new gateway manager.
func NewGatewayManager() *GatewayManager { return &GatewayManager{} }

// RegisterGateway registers the gateway and returns true if successfully,
// or returns false if the gateway has been registered.
//
// Notice: you can use this to manage all the global gateways.
func (m *GatewayManager) RegisterGateway(g *Gateway) (ok bool) {
	_, loaded := m.gateways.LoadOrStore(g.Name(), g)
	return !loaded
}

// UnregisterGateway unregisters the gateway by the name.
func (m *GatewayManager) UnregisterGateway(name string) *Gateway {
	if value, loaded := m.gateways.LoadAndDelete(name); loaded {
		return value.(*Gateway)
	}
	return nil
}

// GetGateway returns the registered gateway by the name.
//
// Return nil if the gateway does not exist.
func (m *GatewayManager) GetGateway(name string) *Gateway {
	if value, ok := m.gateways.Load(name); ok {
		return value.(*Gateway)
	}
	return nil
}

// GetGateways returns all the registered gateways.
func (m *GatewayManager) GetGateways() (gateways []*Gateway) {
	gateways = make([]*Gateway, 0, 8)
	m.gateways.Range(func(key, value interface{}) bool {
		gateways = append(gateways, value.(*Gateway))
		return true
	})
	return
}
