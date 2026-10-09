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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gin_mw "github.com/SENERGY-Platform/gin-middleware"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// usageController answers the usage the way the controller does and records what it was asked.
type usageController struct {
	Controller
	usage   model.ImportTypeUsage
	err     error
	code    int
	asked   string
	askedBy string
}

func (c *usageController) ImportTypeUsage(_ context.Context, token jwt.Token, importTypeId string) (model.ImportTypeUsage, error, int) {
	c.asked, c.askedBy = importTypeId, token.Token
	if c.err != nil {
		return model.ImportTypeUsage{}, c.err, c.code
	}
	return c.usage, nil, http.StatusOK
}

func usageRouter(control Controller) *gin.Engine {
	router := gin.New()
	router.Use(gin_mw.ErrorHandler(model.GetStatusCode, ", "))
	ImportTypeUsageEndpoints(config.Config{}, control, router)
	return router
}

func getUsage(router *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/import-type-usage/urn:infai:ses:import-type:t1", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestImportTypeUsageAnswersCountAndReadable(t *testing.T) {
	control := &usageController{usage: model.ImportTypeUsage{
		Instances: 3,
		Readable:  []model.InstanceRef{{Id: "urn:infai:ses:import:1", Name: "weather"}},
	}}
	auth := tokenWithUsername(t, "someone")
	recorder := getUsage(usageRouter(control), auth)
	if recorder.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", recorder.Code, recorder.Body.String())
	}
	want := `{"instances":3,"readable":[{"id":"urn:infai:ses:import:1","name":"weather"}]}`
	if recorder.Body.String() != want {
		t.Errorf("body = %s, want %s", recorder.Body.String(), want)
	}
	if control.asked != "urn:infai:ses:import-type:t1" || control.askedBy != auth {
		t.Errorf("controller asked for %q with %q", control.asked, control.askedBy)
	}
}

// A zero count must still carry the list as an empty array: the caller decodes it as a list.
func TestImportTypeUsageZeroHasAnEmptyList(t *testing.T) {
	control := &usageController{}
	recorder := getUsage(usageRouter(control), tokenWithUsername(t, "someone"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("answered %d: %s", recorder.Code, recorder.Body.String())
	}
	if want := `{"instances":0,"readable":[]}`; recorder.Body.String() != want {
		t.Errorf("body = %s, want %s", recorder.Body.String(), want)
	}
}

func TestImportTypeUsageNeedsAToken(t *testing.T) {
	for name, authorization := range map[string]string{"no header": "", "not a token": "Bearer not-a-token"} {
		t.Run(name, func(t *testing.T) {
			control := &usageController{}
			recorder := getUsage(usageRouter(control), authorization)
			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("answered %d, want 401: %s", recorder.Code, recorder.Body.String())
			}
			if control.asked != "" {
				t.Errorf("controller was asked for %q without a valid token", control.asked)
			}
		})
	}
}

func TestImportTypeUsageFailureIs500(t *testing.T) {
	control := &usageController{err: errors.New("db down"), code: http.StatusInternalServerError}
	recorder := getUsage(usageRouter(control), tokenWithUsername(t, "someone"))
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("answered %d, want 500: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"instances"`) {
		t.Errorf("a failed lookup must not look like a count: %s", recorder.Body.String())
	}
}
