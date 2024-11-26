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

package controller

import (
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
)

type Controller struct {
	db               Database
	deploymentClient DeploymentClient
	kafkaAdmin       KafkaAdmin
	config           config.Config
	permv2           permV2Client.Client
}

func New(config config.Config, db Database, deploymentClient DeploymentClient, kafkaAdmin KafkaAdmin, perm permV2Client.Client) *Controller {
	return &Controller{
		db:               db,
		deploymentClient: deploymentClient,
		kafkaAdmin:       kafkaAdmin,
		config:           config,
		permv2:           perm,
	}
}
