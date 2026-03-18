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
	"net/http"
	"reflect"
	"runtime"

	"github.com/SENERGY-Platform/service-commons/pkg/accesslog"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/import-deploy/lib/api/util"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"github.com/julienschmidt/httprouter"
)

var endpoints = []func(config config.Config, control Controller, router *httprouter.Router){}

func Start(config config.Config, control Controller) (err error) {
	log.Logger.Info("start api")
	router := httprouter.New()
	log.Logger.Info("add heart beat endpoint")
	router.GET("/", func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		writer.WriteHeader(http.StatusOK)
	})
	for _, e := range endpoints {
		log.Logger.Info("add endpoint", "endpoint", runtime.FuncForPC(reflect.ValueOf(e).Pointer()).Name())
		e(config, control, router)
	}
	log.Logger.Info("add logging and cors")
	corsHandler := util.NewCors(router)
	logger := accesslog.New(corsHandler)
	log.Logger.Info("listen on port", "port", config.ServerPort)
	go func() {
		err := http.ListenAndServe(":"+config.ServerPort, logger)
		if err != nil {
			log.Logger.Error("unable to start api listener", attributes.ErrorKey, err)
		}
	}()
	return nil
}

func getToken(request *http.Request) (token jwt.Token, err error) {
	token, err = jwt.GetParsedToken(request)
	if err != nil {
		return token, err
	}
	return token, nil
}
