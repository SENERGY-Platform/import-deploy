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
	"bytes"
	"encoding/json"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"net/http"
	"strconv"
)

func (c *Client) ListInstances(jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int) {
	if asc {
		sort += ".asc"
	} else {
		sort += ".desc"
	}

	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/instances"+
		"?limit="+strconv.FormatInt(limit, 10)+
		"&offset="+strconv.FormatInt(offset, 10)+
		"&sort="+sort+
		"&search="+search+
		"&exclude_generated="+strconv.FormatBool(!includeGenerated)+
		"&for_user="+forUser,
		nil)
	if err != nil {
		return results, err, http.StatusBadRequest
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	return do[[]model.Instance](req)
}

func (c *Client) ReadInstance(id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int) {
	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/instances/"+id+"&for_user="+forUser, nil)
	if err != nil {
		return result, err, http.StatusBadRequest
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	return do[model.Instance](req)
}

func (c *Client) CreateInstance(instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int) {
	b, err := json.Marshal(instance)
	if err != nil {
		return result, err, http.StatusBadRequest
	}
	req, err := http.NewRequest(http.MethodPost, c.baseUrl+"/instances", bytes.NewBuffer(b))
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	return do[model.Instance](req)
}

func (c *Client) SetInstance(importType model.Instance, jwt jwt.Token) (err error, code int) {
	b, err := json.Marshal(importType)
	if err != nil {
		return err, http.StatusBadRequest
	}
	req, err := http.NewRequest(http.MethodPost, c.baseUrl+"/instances", bytes.NewBuffer(b))
	if err != nil {
		return err, http.StatusInternalServerError
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, resp.StatusCode
}

func (c *Client) DeleteInstance(id string, jwt jwt.Token, forUser string) (err error, errCode int) {
	req, err := http.NewRequest(http.MethodDelete, c.baseUrl+"/instances/"+id+"&for_user="+forUser, nil)
	if err != nil {
		return err, http.StatusBadRequest
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, resp.StatusCode
}

func (c *Client) CountInstances(jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int) {
	req, err := http.NewRequest(http.MethodGet, c.baseUrl+"/instances"+
		"?search="+search+
		"&exclude_generated="+strconv.FormatBool(!includeGenerated),
		nil)
	if err != nil {
		return 0, err, http.StatusBadRequest
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(jwt))
	return do[int64](req)
}
