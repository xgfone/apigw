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

	"github.com/xgfone/apigw"
)

// MiddlewareLoader is used to load the middleware.
type MiddlewareLoader interface {
	Name() string
	Middleware() (apigw.Middleware, error)
}

type middlewareLoader struct {
	name string
	mw   func() (apigw.Middleware, error)
}

// NewMiddlewareLoader returns a new middleware loader.
func NewMiddlewareLoader(name string, plugin func() (apigw.Middleware, error)) MiddlewareLoader {
	return middlewareLoader{name: name, mw: plugin}
}

func (l middlewareLoader) Name() string                          { return l.name }
func (l middlewareLoader) Middleware() (apigw.Middleware, error) { return l.mw() }

var mwLoaders = make(map[string]MiddlewareLoader, 4)

// RegisterMiddlewareLoader registers the middleware loader.
//
// If the loader has been registered, it will panic.
func RegisterMiddlewareLoader(l MiddlewareLoader) {
	name := l.Name()
	if _, ok := mwLoaders[name]; ok {
		panic(fmt.Errorf("the middleware loader named '%s' has been registered", name))
	}
	mwLoaders[name] = l
}

// UnregisterMiddlewareLoader unregisters the middlware loader by the name.
func UnregisterMiddlewareLoader(name string) { delete(mwLoaders, name) }

// GetMiddlewareLoader returns the middleware loader by its name.
//
// Return nil if it does not exist.
func GetMiddlewareLoader(name string) MiddlewareLoader { return mwLoaders[name] }

// GetMiddlewareLoaders returns the name list of all the middleware loaders.
func GetMiddlewareLoaders() []string {
	names := make([]string, 0, len(mwLoaders))
	for name := range mwLoaders {
		names = append(names, name)
	}
	return names
}
