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

	router.GET(resource, func(c *gin.Context) {
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

		includeGenerated := strings.ToLower(c.Query("exclude_generated")) != "true"
		results, err, errCode := control.ListInstances(token, limitInt, offsetInt, orderBy, asc, search, includeGenerated)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.JSON(200, results)
	})

	router.GET("/total"+resource, func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}

		search := c.Query("search")
		includeGenerated := strings.ToLower(c.Query("exclude_generated")) != "true"

		count, err, errCode := control.CountInstances(token, search, includeGenerated)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.String(200, strconv.FormatInt(int64(count), 10))
	})

	router.GET(resource+"/:id", func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		result, err, errCode := control.ReadInstance(id, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.JSON(200, result)
	})

	router.DELETE(resource+"/:id", func(c *gin.Context) {
		token, err := getToken(c.Request)
		if err != nil {
			_ = c.Error(errors.Join(model.ErrBadRequest, err))
			return
		}
		id := c.Param("id")
		err, errCode := control.DeleteInstance(id, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(errCode), err))
			return
		}
		c.Status(errCode)
	})

	router.PUT(resource+"/:id", func(c *gin.Context) {
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
		err, code := control.SetInstance(instance, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.Status(200)
	})

	router.POST(resource, func(c *gin.Context) {
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
		result, err, code := control.CreateInstance(instance, token)
		if err != nil {
			_ = c.Error(errors.Join(model.GetError(code), err))
			return
		}
		c.JSON(code, result)
	})
}
