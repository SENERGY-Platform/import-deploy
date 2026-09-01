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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

func (c *Client) ListInstances(jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int) {
	return c.ListInstancesContext(context.TODO(), jwt, limit, offset, sort, asc, search, includeGenerated, forUser)
}

func (c *Client) ListInstancesContext(ctx context.Context, jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int) {
	if asc {
		sort += ".asc"
	} else {
		sort += ".desc"
	}

	req, err := newRequest(ctx, http.MethodGet, c.baseUrl+"/instances"+
		"?limit="+strconv.FormatInt(limit, 10)+
		"&offset="+strconv.FormatInt(offset, 10)+
		"&sort="+sort+
		"&search="+search+
		"&exclude_generated="+strconv.FormatBool(!includeGenerated)+
		"&for_user="+forUser,
		nil, jwt)
	if err != nil {
		return results, err, http.StatusBadRequest
	}
	return do[[]model.Instance](req)
}

func (c *Client) ReadInstance(id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int) {
	return c.ReadInstanceContext(context.TODO(), id, jwt, forUser)
}

func (c *Client) ReadInstanceContext(ctx context.Context, id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int) {
	req, err := newRequest(ctx, http.MethodGet, c.baseUrl+"/instances/"+id+"&for_user="+forUser, nil, jwt)
	if err != nil {
		return result, err, http.StatusBadRequest
	}
	return do[model.Instance](req)
}

func (c *Client) CreateInstance(instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int) {
	return c.CreateInstanceContext(context.TODO(), instance, jwt)
}

func (c *Client) CreateInstanceContext(ctx context.Context, instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int) {
	b, err := json.Marshal(instance)
	if err != nil {
		return result, err, http.StatusBadRequest
	}
	req, err := newRequest(ctx, http.MethodPost, c.baseUrl+"/instances", bytes.NewBuffer(b), jwt)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return do[model.Instance](req)
}

func (c *Client) SetInstance(importType model.Instance, jwt jwt.Token) (err error, code int) {
	return c.SetInstanceContext(context.TODO(), importType, jwt)
}

func (c *Client) SetInstanceContext(ctx context.Context, importType model.Instance, jwt jwt.Token) (err error, code int) {
	b, err := json.Marshal(importType)
	if err != nil {
		return err, http.StatusBadRequest
	}
	req, err := newRequest(ctx, http.MethodPost, c.baseUrl+"/instances", bytes.NewBuffer(b), jwt)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return doWithoutResult(req)
}

func (c *Client) DeleteInstance(id string, jwt jwt.Token, forUser string) (err error, errCode int) {
	return c.DeleteInstanceContext(context.TODO(), id, jwt, forUser)
}

func (c *Client) DeleteInstanceContext(ctx context.Context, id string, jwt jwt.Token, forUser string) (err error, errCode int) {
	req, err := newRequest(ctx, http.MethodDelete, c.baseUrl+"/instances/"+id+"&for_user="+forUser, nil, jwt)
	if err != nil {
		return err, http.StatusBadRequest
	}
	return doWithoutResult(req)
}

func (c *Client) CountInstances(jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int) {
	return c.CountInstancesContext(context.TODO(), jwt, search, includeGenerated)
}

func (c *Client) CountInstancesContext(ctx context.Context, jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int) {
	req, err := newRequest(ctx, http.MethodGet, c.baseUrl+"/instances"+
		"?search="+search+
		"&exclude_generated="+strconv.FormatBool(!includeGenerated),
		nil, jwt)
	if err != nil {
		return 0, err, http.StatusBadRequest
	}
	return do[int64](req)
}

// doWithoutResult runs a request whose response body carries nothing the caller
// wants. The body is drained and closed all the same, so the connection goes back
// to the pool instead of being dropped.
func doWithoutResult(req *http.Request) (err error, code int) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil, resp.StatusCode
}
