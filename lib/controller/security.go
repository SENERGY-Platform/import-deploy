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
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/permission-search/lib/client"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

func (this *Controller) checkBool(token jwt.Token, kind string, id string, action model.AuthAction) (allowed bool, err error) {
	if token.IsAdmin() {
		return true, nil
	}
	err = this.permissionsearch.CheckUserOrGroup(token.Jwt(), kind, id, action.String())
	switch err {
	case nil:
		return true, nil
	case client.ErrAccessDenied:
		return false, nil
	case client.ErrNotFound:
		return false, nil
	default:
		return false, err
	}
}
