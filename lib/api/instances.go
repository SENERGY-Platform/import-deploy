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
	"errors"
	"strconv"
	"strings"

	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/gin-gonic/gin"
)

func init() {
	endpoints = append(endpoints, InstancesEndpoints)
}

func InstancesEndpoints(_ config.Config, control Controller, router *gin.Engine) {
	resource := "/instances"

	router.GET(resource, listInstancesHandler(control))
	router.GET("/total"+resource, countInstancesHandler(control))
	router.GET(resource+"/:id", readInstanceHandler(control))
	router.DELETE(resource+"/:id", deleteInstanceHandler(control))
	router.PUT(resource+"/:id", setInstanceHandler(control))
	router.POST(resource, createInstanceHandler(control))
}

// listInstancesHandler godoc
// @Summary List instances
// @Description Returns import instances visible to the caller, including the current container status.
// @Tags instances
// @Produce json
// @Param limit query int false "Maximum number of results" default(100)
// @Param offset query int false "Result offset" default(0)
// @Param sort query string false "Sort order" default(name)
// @Param search query string false "Free-text search term"
// @Param exclude_generated query bool false "If true, excludes generated instances"
// @Param ids query []string false "Get only specific instances by id (comma-separated list)"
// @Success 200 {array} model.Instance
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /instances [get]
func listInstancesHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		limit := c.Query("limit")
		if limit == "" {
			limit = "100"
		}
		limitInt, err := strconv.ParseInt(limit, 10, 64)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		offset := c.Query("offset")
		if offset == "" {
			offset = "0"
		}
		offsetInt, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		sort := c.Query("sort")
		if sort == "" {
			sort = "name"
		}
		orderBy := strings.Split(sort, ".")[0]
		asc := !strings.HasSuffix(sort, ".desc")

		search := c.Query("search")

		var ids []string
		if c.Query("ids") != "" {
			ids = strings.Split(c.Query("ids"), ",")
		}

		includeGenerated := strings.ToLower(c.Query("exclude_generated")) != "true"
		results, err, errCode := control.ListInstances(c.Request.Context(), token, limitInt, offsetInt, orderBy, asc, search, includeGenerated, ids)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.JSON(200, results)
	}
}

// countInstancesHandler godoc
// @Summary Count instances
// @Description Returns the total number of instances visible to the caller.
// @Tags instances
// @Produce plain
// @Param search query string false "Free-text search term"
// @Param exclude_generated query bool false "If true, excludes generated instances"
// @Success 200 {string} string
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /total/instances [get]
func countInstancesHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}

		search := c.Query("search")
		includeGenerated := strings.ToLower(c.Query("exclude_generated")) != "true"

		count, err, errCode := control.CountInstances(c.Request.Context(), token, search, includeGenerated)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.String(200, strconv.FormatInt(int64(count), 10))
	}
}

// readInstanceHandler godoc
// @Summary Get instance
// @Description Returns a single instance by id, including the current container status.
// @Tags instances
// @Produce json
// @Param id path string true "Instance id"
// @Success 200 {object} model.Instance
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 404 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /instances/{id} [get]
func readInstanceHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		result, err, errCode := control.ReadInstance(c.Request.Context(), id, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.JSON(200, result)
	}
}

// deleteInstanceHandler godoc
// @Summary Delete instance
// @Description Deletes a single instance by id.
// @Tags instances
// @Param id path string true "Instance id"
// @Success 200
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 404 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /instances/{id} [delete]
func deleteInstanceHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		err, errCode := control.DeleteInstance(c.Request.Context(), id, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.Status(errCode)
	}
}

// setInstanceHandler godoc
// @Summary Update instance
// @Description Replaces an instance. The request body id must match the path id.
// @Tags instances
// @Accept json
// @Param id path string true "Instance id"
// @Param instance body model.Instance true "Full instance payload"
// @Success 200
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 404 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /instances/{id} [put]
func setInstanceHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		instance := model.Instance{}
		err = c.ShouldBind(&instance)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}

		if id != instance.Id {
			_ = c.Error(errors.Join(model.ErrBadRequest, errors.New("IDs don't match")))
			return
		}
		err, code := control.SetInstance(c.Request.Context(), instance, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Status(200)
	}
}

// createInstanceHandler godoc
// @Summary Create instance
// @Description Creates a new instance.
// @Tags instances
// @Accept json
// @Produce json
// @Param instance body model.Instance true "Instance payload"
// @Success 200 {object} model.Instance
// @Failure 400 {string} ErrorResponse
// @Failure 403 {string} ErrorResponse
// @Failure 404 {string} ErrorResponse
// @Failure 500 {string} ErrorResponse
// @Router /instances [post]
func createInstanceHandler(control Controller) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		instance := model.Instance{}
		err = c.ShouldBind(&instance)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		result, err, code := control.CreateInstance(c.Request.Context(), instance, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.JSON(code, result)
	}
}
