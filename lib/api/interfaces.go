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
	"context"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

// The ctx of every method is the one of the incoming request: it carries the trace
// and the baggage the OpenTelemetry middleware put on it.
type Controller interface {
	ListInstances(ctx context.Context, jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, ids []string) (results []model.Instance, err error, errCode int)
	ReadInstance(ctx context.Context, id string, jwt jwt.Token) (result model.Instance, err error, errCode int)
	CreateInstance(ctx context.Context, instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int)
	SetInstance(ctx context.Context, importType model.Instance, jwt jwt.Token) (err error, code int)
	DeleteInstance(ctx context.Context, id string, jwt jwt.Token) (err error, errCode int)
	CountInstances(ctx context.Context, jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int)
}
