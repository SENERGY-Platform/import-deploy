/*
 * Copyright 2026 InfAI (CC SES)
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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// recordingController keeps the instance a handler hands over.
type recordingController struct {
	Controller
	created *model.Instance
	set     *model.Instance
}

func (c *recordingController) CreateInstance(_ context.Context, instance model.Instance, _ jwt.Token) (model.Instance, error, int) {
	c.created = &instance
	return instance, nil, http.StatusOK
}

func (c *recordingController) SetInstance(_ context.Context, instance model.Instance, _ jwt.Token) (error, int) {
	c.set = &instance
	return nil, http.StatusOK
}

// The smart service import worker sends its json without a Content-Type header.
func TestInstanceBodiesAreJSONWithoutAContentType(t *testing.T) {
	control := &recordingController{}
	router := gin.New()
	router.POST("/instances", createInstanceHandler(control))
	router.PUT("/instances/:id", setInstanceHandler(control))
	for _, contentType := range []string{"", "application/json"} {
		body := `{"id":"urn:infai:ses:import:1","import_type_id":"urn:infai:ses:import-type:t","name":"n"}`
		requests := []*http.Request{
			httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader(strings.Replace(body, "urn:infai:ses:import:1", "", 1))),
			httptest.NewRequest(http.MethodPut, "/instances/urn:infai:ses:import:1", strings.NewReader(body)),
		}
		for _, req := range requests {
			req.Header.Set("Authorization", tokenWithUsername(t, "someone"))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s with Content-Type %q answered %d: %s", req.Method, contentType, recorder.Code, recorder.Body.String())
			}
		}
		if control.created == nil || control.created.ImportTypeId != "urn:infai:ses:import-type:t" {
			t.Fatalf("Content-Type %q: create got %+v", contentType, control.created)
		}
		if control.set == nil || control.set.ImportTypeId != "urn:infai:ses:import-type:t" {
			t.Fatalf("Content-Type %q: set got %+v", contentType, control.set)
		}
		control.created, control.set = nil, nil
	}
}
