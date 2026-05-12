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

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"

	gin_mw "github.com/SENERGY-Platform/gin-middleware"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

var endpoints = []func(config config.Config, control Controller, router *gin.Engine){}

// Start godoc
// @title Import Deploy API
// @description Launches and stops import instances and provides information about them.
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func Start(config config.Config, control Controller) (err error) {
	log.Logger.Info("start api")
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(
		cors.New(cors.Config{
			AllowAllOrigins: true,
			AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:    []string{"Authorization", "Content-Type"},
		}),
		gin_mw.StructLoggerHandlerWithDefaultGenerators(
			log.Logger.With(attributes.LogRecordTypeKey, attributes.HttpAccessLogRecordTypeVal),
			attributes.Provider,
			[]string{},
			nil,
		),
		requestid.New(requestid.WithCustomHeaderStrKey("X-Request-ID")),
		gin_mw.ErrorHandler(model.GetStatusCode, ", "),
		gin_mw.StructRecoveryHandler(log.Logger, gin_mw.DefaultRecoveryFunc),
	)
	for _, e := range endpoints {
		log.Logger.Info("add endpoint", "name", runtime.FuncForPC(reflect.ValueOf(e).Pointer()).Name())
		e(config, control, router)
	}
	router.GET("/", healthHandler)
	log.Logger.Info("listen on port", "port", config.ServerPort)
	go func() {
		err := http.ListenAndServe(":"+config.ServerPort, router)
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

// healthHandler godoc
// @Summary Health check
// @Description Returns HTTP 200 when the service is running.
// @Tags service
// @Success 200
// @Router / [get]
func healthHandler(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}
