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
	"io"
)

// BackendGroupUpdater is used to update the backend from the group.
type BackendGroupUpdater interface {
	Name() string
	AddBackendFromGroup(b Backend)
	DelBackendFromGroup(b Backend)
}

// BackendGroup is used to manage a group of Backend.
type BackendGroup interface {
	io.Closer

	// Name returns the name of the group.
	Name() string

	// AddBackend adds the backend into the group.
	//
	// If the backend has been added, do nothing.
	AddBackend(Backend)

	// DelBackend deletes the backend from the group.
	//
	// If the backend does not exist, do nothing.
	DelBackend(Backend)

	// GetBackends returns all the backends in the group.
	GetBackends() []Backend

	// GetUpdater returns the backend updater by the name.
	//
	// If the backend updater does not exist, return nil.
	GetUpdater(name string) BackendGroupUpdater

	// GetUpdaters returns all the backend updaters.
	GetUpdaters() []BackendGroupUpdater

	// AddUpdater adds the backend updater, which should add all the backends
	// into the updater.
	//
	// If the updater has been added, do nothing.
	AddUpdater(BackendGroupUpdater)

	// DelUpdater deletes the backend updater, which should deletes
	// all the backends from the updater.
	//
	// If the updater does not exist, do nothing.
	DelUpdater(BackendGroupUpdater)
}
