/*
 * Copyright 2023 InfAI (CC SES)
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

package client

import (
	"encoding/json"
	"fmt"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"io"
	"net/http"
)

type Interface interface {
	ListInstances(jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int)
	ReadInstance(id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int)
	CreateInstance(instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int)
	SetInstance(importType model.Instance, jwt jwt.Token) (err error, code int)
	DeleteInstance(id string, jwt jwt.Token, forUser string) (err error, errCode int)
	CountInstances(jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int)
}

type Client struct {
	baseUrl string
}

func NewClient(baseUrl string) Interface {
	return &Client{baseUrl: baseUrl}
}

func do[T any](req *http.Request) (result T, err error, code int) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		temp, _ := io.ReadAll(resp.Body) //read error response end ensure that resp.Body is read to EOF
		return result, fmt.Errorf("unexpected statuscode %v: %v", resp.StatusCode, string(temp)), resp.StatusCode
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		_, _ = io.ReadAll(resp.Body) //ensure resp.Body is read to EOF
		return result, err, http.StatusInternalServerError
	}
	return
}
