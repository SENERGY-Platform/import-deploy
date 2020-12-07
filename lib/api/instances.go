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
)

func init() {
	endpoints = append(endpoints, InstancesEndpoints)
}

func InstancesEndpoints(config config.Config, control Controller, router *jwt_http_router.Router) {
	resource := "/instances"

	router.GET(resource, func(writer http.ResponseWriter, request *http.Request, params jwt_http_router.Params, jwt jwt_http_router.Jwt) {
		results, err, errCode := control.ListInstances(jwt, 0, 0, "") // TODO
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
		id := params.ByName("id")
		result, err, errCode := control.ReadInstance(id, jwt)
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
