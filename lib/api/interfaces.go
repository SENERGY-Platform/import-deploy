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
	"github.com/SENERGY-Platform/import-deploy/lib/auth"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
)

type Controller interface {
	ListInstances(jwt auth.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool) (results []model.Instance, err error, errCode int)
	ReadInstance(id string, jwt auth.Token) (result model.Instance, err error, errCode int)
	CreateInstance(instance model.Instance, jwt auth.Token) (result model.Instance, err error, code int)
	SetInstance(importType model.Instance, jwt auth.Token) (err error, code int)
	DeleteInstance(id string, jwt auth.Token) (err error, errCode int)
	CountInstances(jwt auth.Token, search string, includeGenerated bool) (count int64, err error, errCode int)
}
