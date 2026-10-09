/*
 * Copyright 2026 InfAI (CC SES)
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

package api

import (
	"errors"
	"net/http"

	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/gin-gonic/gin"
)

func init() {
	endpoints = append(endpoints, ImportTypeUsageEndpoints)
}

func ImportTypeUsageEndpoints(_ config.Config, control Controller, router *gin.Engine) {
	router.GET("/import-type-usage/:id", importTypeUsageHandler(control))
}

// importTypeUsageHandler godoc
// @Summary Get import type usage
// @Description Returns how many import instances of all users use the import type, and which of them the caller may read. The import repository asks this before it deletes an import type.
// @Tags instances
// @Produce json
// @Param id path string true "Import type id"
// @Success 200 {object} model.ImportTypeUsage
// @Failure 401 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /import-type-usage/{id} [get]
func importTypeUsageHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrUnauthorized, err))
			return
		}
		usage, err, errCode := control.ImportTypeUsage(c.Request.Context(), token, c.Param("id"))
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		if usage.Readable == nil {
			usage.Readable = []model.InstanceRef{}
		}
		c.JSON(http.StatusOK, usage)
	}
}
