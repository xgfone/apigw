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

package loader

import (
	"fmt"

	"github.com/xgfone/apigw/sd"
)

// ServiceDiscoveryLoader is used to load the service discovery.
type ServiceDiscoveryLoader interface {
	ServiceDiscovery() (sd.ServiceDiscovery, error)
	Name() string
}

type sdLoader struct {
	sd   func() (sd.ServiceDiscovery, error)
	name string
}

// NewServiceDiscoveryLoader returns a new service discovery loader.
func NewServiceDiscoveryLoader(name string, sd func() (sd.ServiceDiscovery, error)) ServiceDiscoveryLoader {
	return sdLoader{name: name, sd: sd}
}

func (l sdLoader) Name() string                                   { return l.name }
func (l sdLoader) ServiceDiscovery() (sd.ServiceDiscovery, error) { return l.sd() }

var sdLoaders = make(map[string]ServiceDiscoveryLoader, 4)

// RegisterServiceDiscoveryLoader registers the service discovery loader.
//
// If the loader has been registered, it will panic.
func RegisterServiceDiscoveryLoader(l ServiceDiscoveryLoader) {
	name := l.Name()
	if _, ok := sdLoaders[name]; ok {
		panic(fmt.Errorf("the service discovery loader named '%s' has been registered", name))
	}
	sdLoaders[name] = l
}

// UnregisterServiceDiscoveryLoader unregisters the service discovery loader by the name.
func UnregisterServiceDiscoveryLoader(name string) { delete(sdLoaders, name) }

// GetServiceDiscoveryLoader returns the service discovery loader by its name.
//
// Return nil if it does not exist.
func GetServiceDiscoveryLoader(name string) ServiceDiscoveryLoader { return sdLoaders[name] }

// GetServiceDiscoveryLoaders returns the name list of all the service discovery loaders.
func GetServiceDiscoveryLoaders() []string {
	names := make([]string, 0, len(sdLoaders))
	for name := range sdLoaders {
		names = append(names, name)
	}
	return names
}
