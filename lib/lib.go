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
	"errors"
	"github.com/SENERGY-Platform/import-deploy/lib/api"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/controller"
	"github.com/SENERGY-Platform/import-deploy/lib/database/mongo"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy/dockerClient"
	rancher_api "github.com/SENERGY-Platform/import-deploy/lib/deploy/rancher-api"
	rancher2_api "github.com/SENERGY-Platform/import-deploy/lib/deploy/rancher2-api"
	kafkaAdmin "github.com/SENERGY-Platform/import-deploy/lib/kafka-admin"
	"log"
)

func Start(conf config.Config) (stop func(), err error) {

	data, err := mongo.New(conf)
	if err != nil {
		return stop, err
	}

	var deploymentClient deploy.DeploymentClient

	switch (conf.DeployMode) {
	case "docker":
		deploymentClient, err = dockerClient.New(conf)
		break
	case "rancher1":
		deploymentClient = rancher_api.New(conf)
		break
	case "rancher2":
		deploymentClient = rancher2_api.New(conf)
		break
	default:
		data.Disconnect()
		return stop, errors.New("unknown deploy_mode")
	}
	if err != nil {
		data.Disconnect()
		return stop, err
	}

	kafka, err := kafkaAdmin.New(conf)
	if err != nil {
		data.Disconnect()
		_ = deploymentClient.Disconnect() // best effort
		return stop, err
	}

	ctrl := controller.New(conf, data, deploymentClient, kafka)

	err = api.Start(conf, ctrl)
	if err != nil {
		data.Disconnect();
		_ = deploymentClient.Disconnect() // best effort
		_ = kafka.Disconnect() // best effort
		log.Println("ERROR: unable to start api", err)
		return stop, err
	}

	return func() {
		_ = deploymentClient.Disconnect() // best effort
		_ = kafka.Disconnect() // best effort
		data.Disconnect()
	}, err
}
