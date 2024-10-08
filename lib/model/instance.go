/*
 * Copyright 2020 InfAI (CC SES)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package model

import "time"
import permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"

type Instances []Instance

type Instance struct {
	Id           string           `json:"id"`
	Name         string           `json:"name"`
	ImportTypeId string           `json:"import_type_id"`
	Image        string           `json:"image"`
	KafkaTopic   string           `json:"kafka_topic"`
	Configs      []InstanceConfig `json:"configs"`
	Restart      *bool            `json:"restart"`
	ServiceId    string           `json:"-"`
	Owner        string           `json:"-"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	Generated    bool             `json:"generated"`
}

type InstanceConfig struct {
	Name        string      `json:"name"`
	Value       interface{} `json:"value"`
	ValueString *string     `json:"-"`
}

const PermV2InstanceTopic = "import-instances"

func SetDefaultPermissions(instance Instance, permissions permV2Client.ResourcePermissions) {
	permissions.UserPermissions[instance.Owner] = permV2Client.PermissionsMap{
		Read:         true,
		Write:        true,
		Execute:      true,
		Administrate: true,
	}
}
