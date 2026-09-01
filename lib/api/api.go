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
	"fmt"
	"net/http"
	"reflect"
	"runtime"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"

	gin_mw "github.com/SENERGY-Platform/gin-middleware"
	"github.com/SENERGY-Platform/gin-middleware/otelx"
	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

var endpoints = []func(config config.Config, control Controller, router *gin.Engine){}

// ServiceName identifies this service to the outside: it names the traces in the
// collector and the tracer that produces them.
const ServiceName = "import-deploy"

// Start godoc
// @title Import Deploy API
// @description Launches and stops import instances and provides information about them.
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func Start(ctx context.Context, config config.Config, control Controller) (err error) {
	log.Logger.Info("start api")
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	otelHandler, err := otelx.GinOpenTelemetry(ctx, ServiceName, config.OtelEndpoint)
	if err != nil {
		return fmt.Errorf("failed to set up OpenTelemetry: %w", err)
	}

	router.Use(
		cors.New(cors.Config{
			AllowAllOrigins: true,
			AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:    []string{"Authorization", "Content-Type"},
		}),
		// Before everything that reads a context: it extracts the trace and the baggage
		// off the request and puts them onto the request context, which the access log,
		// the handlers and the controller read from.
		otelHandler,
		// Directly after it, and it has to stay there: see DiscardBaggageErrors.
		DiscardBaggageErrors(),
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

// DiscardBaggageErrors takes the errors the OpenTelemetry handler reported off the
// request and logs them instead.
//
// otelx reports a baggage value it cannot carry -- one holding a space, a comma or a
// non-ASCII character -- with gin's c.Error. gin_mw.ErrorHandler then turns anything
// in c.Errors into a response: it forces a 500 where the status was below 400, and
// appends the error text to the body. A user whose Keycloak username holds a space
// would therefore get a 500 on every DELETE and a corrupted JSON body on every GET,
// for a log annotation that failed.
//
// This handler has to sit immediately after the OpenTelemetry handler. otelx adds
// those errors before it calls c.Next(), so at this point in the chain nothing else
// can have added one, which is what makes clearing the slice safe. Moved further
// down, it would discard a real handler's error.
//
// The proper fix belongs in gin-middleware, which should not use c.Error for
// something that is not a request error.
func DiscardBaggageErrors() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Errors) > 0 {
			for _, reported := range c.Errors {
				log.Logger.WarnContext(c.Request.Context(),
					"could not put a value into the request baggage", attributes.ErrorKey, reported.Err)
			}
			c.Errors = nil
		}
		c.Next()
	}
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
