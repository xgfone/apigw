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

package backend

import (
	"fmt"

	"github.com/xgfone/apigw"
	"github.com/xgfone/apigw/forward/lb"
)

var builders = make(map[string]Builder, 4)

// BuilderContext is the context of the builder.
type BuilderContext struct {
	apigw.Route
	lb.HealthCheck
	UserData interface{}
	MetaData map[string]interface{}
}

// Builder is used to build the backend.
type Builder interface {
	Type() string
	New(BuilderContext) (lb.Backend, error)
}

// RegisterBuilder registers the backend builder.
//
// If the builder has been registered, it will panic.
func RegisterBuilder(builder Builder) {
	_type := builder.Type()
	if _, ok := builders[_type]; ok {
		panic(fmt.Errorf("the backend builder typed '%s' has been registered", _type))
	}
	builders[_type] = builder
}

// UnregisterBuilder unregisters the backend builder by the type.
func UnregisterBuilder(_type string) { delete(builders, _type) }

// GetBuilder returns the backend builder by the type.
//
// Return nil if the backend builder does not exist.
func GetBuilder(_type string) Builder { return builders[_type] }

// GetBuilders returns all the backend builders.
func GetBuilders() []Builder {
	bs := make([]Builder, 0, len(builders))
	for _, b := range builders {
		bs = append(bs, b)
	}
	return bs
}

// NewBuilder returns a new builder.
func NewBuilder(_type string, new func(BuilderContext) (lb.Backend, error)) Builder {
	return builder{typ: _type, new: new}
}

type builder struct {
	typ string
	new func(BuilderContext) (lb.Backend, error)
}

func (b builder) Type() string { return b.typ }
func (b builder) New(c BuilderContext) (lb.Backend, error) {
	return b.new(c)
}
