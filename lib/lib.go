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

package lib

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/SENERGY-Platform/import-deploy/lib/api"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/controller"
	"github.com/SENERGY-Platform/import-deploy/lib/database"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy/dockerClient"
	kubernetes_api "github.com/SENERGY-Platform/import-deploy/lib/deploy/kubernetes-api"
	rancher_api "github.com/SENERGY-Platform/import-deploy/lib/deploy/rancher-api"
	rancher2_api "github.com/SENERGY-Platform/import-deploy/lib/deploy/rancher2-api"
	kafkaAdmin "github.com/SENERGY-Platform/import-deploy/lib/kafka-admin"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
)

func Start(conf config.Config, ctx context.Context) (wg *sync.WaitGroup, err error) {
	wg = &sync.WaitGroup{}
	perm := permV2Client.New(conf.PermissionV2Url)
	data, err := database.New(conf, perm, ctx, wg)
	if err != nil {
		return wg, err
	}

	var deploymentClient deploy.DeploymentClient

	switch conf.DeployMode {
	case "docker":
		deploymentClient, err = dockerClient.New(conf, ctx, wg)
	case "rancher1":
		deploymentClient = rancher_api.New(conf)
	case "rancher2":
		deploymentClient = rancher2_api.New(conf)
	case "kubernetes":
		deploymentClient, err = kubernetes_api.New(conf)
	default:
		return wg, errors.New("unknown deploy_mode")
	}
	if err != nil {
		return wg, err
	}

	kafka, err := kafkaAdmin.New(conf)
	if err != nil {
		return wg, err
	}

	ctrl := controller.New(conf, data, deploymentClient, kafka, perm)

	if conf.StartupEnsureDeployed {
		log.Println("Restoring missing import containers")
		err = ctrl.EnsureAllInstancesDeployed()
		if err != nil {
			return wg, err
		}
	}

	err = api.Start(conf, ctrl)
	if err != nil {
		log.Println("ERROR: unable to start api", err)
		return wg, err
	}

	return wg, err
}
