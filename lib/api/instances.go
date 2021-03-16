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
	"encoding/json"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	jwt_http_router "github.com/SmartEnergyPlatform/jwt-http-router"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func init() {
	endpoints = append(endpoints, InstancesEndpoints)
}

func InstancesEndpoints(conf config.Config, control Controller, router *jwt_http_router.Router) {
	resource := "/instances"

	router.GET(resource, func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		var userId string
		if conf.XUserIdForReadAccess {
			userId = request.Header.Get("X-UserId")
			if userId == "" {
				userId = jwt.UserId
			}
		} else {
			userId = jwt.UserId
		}
		limit := request.URL.Query().Get("limit")
		if limit == "" {
			limit = "100"
		}
		limitInt, err := strconv.ParseInt(limit, 10, 64)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		offset := request.URL.Query().Get("offset")
		if offset == "" {
			offset = "0"
		}
		offsetInt, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		sort := request.URL.Query().Get("sort")
		if sort == "" {
			sort = "name"
		}
		orderBy := strings.Split(sort, ".")[0]
		asc := !strings.HasSuffix(sort, ".desc")

		search := request.URL.Query().Get("search")

		includeGenerated := strings.ToLower(request.URL.Query().Get("exclude_generated")) != "true"
		results, err, errCode := control.ListInstances(userId, limitInt, offsetInt, orderBy, asc, search, includeGenerated)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(results)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	})

	router.GET(resource+"/:id", func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		var userId string
		if conf.XUserIdForReadAccess {
			userId = request.Header.Get("X-UserId")
			if userId == "" {
				userId = jwt.UserId
			}
		} else {
			userId = jwt.UserId
		}
		id := params.ByName("id")
		result, err, errCode := control.ReadInstance(id, userId)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
		}
		return
	})

	router.DELETE(resource+"/:id", func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		id := params.ByName("id")
		err, errCode := control.DeleteInstance(id, jwt)
		if err != nil {
			http.Error(writer, err.Error(), errCode)
			return
		}
		writer.WriteHeader(errCode)
		return
	})

	router.PUT(resource+"/:id", func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		id := params.ByName("id")
		instance := model.Instance{}
		err := json.NewDecoder(request.Body).Decode(&instance)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		if id != instance.Id {
			http.Error(writer, "IDs don't match", http.StatusBadRequest)
			return
		}
		err, code := control.SetInstance(instance, jwt)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
		writer.WriteHeader(http.StatusOK)
	})

	router.POST(resource, func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		instance := model.Instance{}
		err := json.NewDecoder(request.Body).Decode(&instance)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		result, err, code := control.CreateInstance(instance, jwt)
		if err != nil {
			http.Error(writer, err.Error(), code)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(code)
		err = json.NewEncoder(writer).Encode(result)
		if err != nil {
			log.Println("ERROR: unable to encode response", err)
			return
		}
		return
	})
}
