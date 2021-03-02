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
	"errors"
	"time"

	"github.com/xgfone/go-service/loadbalancer"
)

// HealthCheck is the information of the health check of the endpoint.
type HealthCheck struct {
	Timeout  time.Duration `json:"timeout,omitempty" xml:"timeout,omitempty"`
	Interval time.Duration `json:"interval,omitempty" xml:"interval,omitempty"`
	RetryNum int           `json:"retrynum,omitempty" xml:"retrynum,omitempty"`
}

// Backend represents the forwarded backend used by Forwarder.
//
// Notice: The Forwarder backend must implement the interface.
type Backend interface {
	loadbalancer.Endpoint
	HealthCheck() HealthCheck
}

// NewBackendWithHealthCheck returns a new backend with the new HealthCheck.
func NewBackendWithHealthCheck(b Backend, hc HealthCheck) Backend {
	return hcBackend{Backend: b, hc: hc}
}

type hcBackend struct {
	hc HealthCheck
	Backend
}

func (b hcBackend) Unwrap() loadbalancer.Endpoint { return b.Backend }
func (b hcBackend) HealthCheck() HealthCheck      { return b.hc }

//////////////////////////////////////////////////////////////////////////////

type endpointBackend struct {
	loadbalancer.Provider
	loadbalancer.Endpoint
	online bool
}

func newEndpointBackend(p loadbalancer.Provider, ep loadbalancer.Endpoint,
	online bool) endpointBackend {
	return endpointBackend{Provider: p, Endpoint: ep, online: online}
}

func (eb endpointBackend) Unwrap() loadbalancer.Endpoint {
	return eb.Endpoint
}

func (eb endpointBackend) HealthCheck() HealthCheck {
	ep := eb.Endpoint
	if eu, ok := eb.Endpoint.(loadbalancer.EndpointUnwrap); ok {
		ep = eu.Unwrap()
	}
	return ep.(Backend).HealthCheck()
}

func (eb endpointBackend) IsHealthy(context.Context) bool {
	return eb.online
}

func (eb endpointBackend) MetaData() map[string]interface{} {
	md := eb.Endpoint.MetaData()
	md["online"] = eb.online
	return md
}

/////////////////////////////////////////////////////////////////////////////

// GroupBackend is the group backend.
type GroupBackend interface {
	BackendGroup() *BackendGroup
	Backend
}

// GroupBackendConfig is used to configure the group backend.
type GroupBackendConfig struct {
	UserData     interface{}
	HealthCheck  HealthCheck
	BackendGroup *BackendGroup
}

// NewGroupBackend returns a new group backend.
func NewGroupBackend(name string, config *GroupBackendConfig) (GroupBackend, error) {
	if name == "" {
		return nil, errors.New("the group name must not be empty")
	}

	var conf GroupBackendConfig
	if config != nil {
		conf = *config
	}

	return groupBackend{Endpoint: loadbalancer.NewNoopEndpoint(name), conf: conf}, nil
}

type groupBackend struct {
	conf GroupBackendConfig
	loadbalancer.Endpoint
}

func (b groupBackend) Type() string                { return "group" }
func (b groupBackend) BackendGroup() *BackendGroup { return b.conf.BackendGroup }
func (b groupBackend) HealthCheck() HealthCheck    { return b.conf.HealthCheck }
func (b groupBackend) UserData() interface{}       { return b.conf.UserData }
func (b groupBackend) MetaData() map[string]interface{} {
	return map[string]interface{}{"name": b.String()}
}
