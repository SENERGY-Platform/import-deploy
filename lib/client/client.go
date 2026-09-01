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

// Package client talks to import-deploy.
//
// Every call has a Context variant. Use it: it carries the caller's trace and
// baggage onto the wire, which is what lets import-deploy -- and the import
// container it deploys -- log under the context of whatever caused the call. A
// smart service worker that passes its instance id this way gets it back on every
// log line the resulting import ever writes.
//
// The variants without a context are kept so existing callers still build. They
// pass context.TODO(), so import-deploy starts a trace of its own and the
// connection to the caller is lost.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/SENERGY-Platform/gin-middleware/otelx"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

type Interface interface {
	ListInstances(jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int)
	ListInstancesContext(ctx context.Context, jwt jwt.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, forUser string) (results []model.Instance, err error, errCode int)
	ReadInstance(id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int)
	ReadInstanceContext(ctx context.Context, id string, jwt jwt.Token, forUser string) (result model.Instance, err error, errCode int)
	CreateInstance(instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int)
	CreateInstanceContext(ctx context.Context, instance model.Instance, jwt jwt.Token) (result model.Instance, err error, code int)
	SetInstance(importType model.Instance, jwt jwt.Token) (err error, code int)
	SetInstanceContext(ctx context.Context, importType model.Instance, jwt jwt.Token) (err error, code int)
	DeleteInstance(id string, jwt jwt.Token, forUser string) (err error, errCode int)
	DeleteInstanceContext(ctx context.Context, id string, jwt jwt.Token, forUser string) (err error, errCode int)
	CountInstances(jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int)
	CountInstancesContext(ctx context.Context, jwt jwt.Token, search string, includeGenerated bool) (count int64, err error, errCode int)
}

type Client struct {
	baseUrl string
}

func NewClient(baseUrl string) Interface {
	return &Client{baseUrl: baseUrl}
}

// newRequest builds a request that carries the caller's trace and baggage. The
// injection is best-effort: a context that cannot be spelled as a header costs the
// correlation, not the call.
func newRequest(ctx context.Context, method string, url string, body io.Reader, token jwt.Token) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", prefixTokenIfNeeded(token))
	_ = otelx.InjectContextToRequest(ctx, req)
	return req, nil
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

func prefixTokenIfNeeded(jwt jwt.Token) string {
	s := jwt.Jwt()
	if !strings.HasPrefix(strings.ToLower(s), "bearer ") {
		s = "Bearer " + s
	}
	return s
}
