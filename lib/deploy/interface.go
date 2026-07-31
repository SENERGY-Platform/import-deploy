/*
 * Copyright 2020 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package deploy

import (
	"errors"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
)

// ErrNotSupported is returned by deployment clients that cannot provide the requested information.
var ErrNotSupported = errors.New("operation not supported by deployment client")

type DeploymentClient interface {
	CreateContainer(name string, image string, env map[string]string, restart bool, userid string, importTypeId string) (id string, err error)
	UpdateContainer(id string, name string, image string, env map[string]string, restart bool, userid string, importTypeId string, existingRestart bool) (newId string, err error)
	RemoveContainer(id string) (err error)
	ContainerExists(id string, restart *bool) (exists bool, err error)
	// GetContainerStatus returns the current status of a single container.
	GetContainerStatus(id string, restart *bool) (status model.InstanceStatus, err error)
	// GetContainerStatuses returns the status of all known containers, keyed by container id.
	GetContainerStatuses() (statuses map[string]model.InstanceStatus, err error)
	Disconnect() (err error)
}
