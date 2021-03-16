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

package api

import (
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SmartEnergyPlatform/jwt-http-router"
)

type Controller interface {
	ListInstances(userId string, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool) (results []model.Instance, err error, errCode int)
	ReadInstance(id string, userId string) (result model.Instance, err error, errCode int)
	CreateInstance(instance model.Instance, jwt jwt_http_router.Jwt) (result model.Instance, err error, code int)
	SetInstance(importType model.Instance, jwt jwt_http_router.Jwt) (err error, code int)
	DeleteInstance(id string, jwt jwt_http_router.Jwt) (err error, errCode int)
}
