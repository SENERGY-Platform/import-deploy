/*
 * Copyright 2020 InfAI (CC SES)
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

package rancher_api

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/httpreq"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/hashicorp/go-uuid"
)

type Rancher struct {
	url       string
	accessKey string
	secretKey string
	stackId   string
}

func New(config config.Config) *Rancher {
	return &Rancher{config.RancherUrl, config.RancherAccessKey, config.RancherSecretKey, config.RancherStackId}
}

// do sends a request to the Rancher API with the credentials, the trace and the
// baggage of ctx attached.
func (r Rancher) do(ctx context.Context, method string, url string, body any) (httpreq.Response, error) {
	return httpreq.Do(ctx, httpreq.Request{
		Method:    method,
		URL:       url,
		Body:      body,
		BasicAuth: &httpreq.BasicAuth{User: r.accessKey, Password: r.secretKey},
	})
}

func (r Rancher) CreateContainer(ctx context.Context, name string, image string, env map[string]string, restart bool, _ string, _ string, bag map[string]string) (id string, err error) {
	id, err, _ = r.createContainer(ctx, name, image, env, restart, bag)
	return id, err
}

func (r Rancher) createContainer(ctx context.Context, name string, image string, env map[string]string, restart bool, bag map[string]string) (id string, err error, code int) {
	labels := map[string]string{
		"io.rancher.container.pull_image":          "always",
		"io.rancher.scheduler.affinity:host_label": "role=worker",
	}
	if !restart {
		labels["io.rancher.container.start_once"] = "true"
	}
	labels = baggage.AddLabels(ctx, labels, bag)

	reqBody := &Request{
		Type:          "service",
		Name:          name,
		StackId:       r.stackId,
		Scale:         1,
		StartOnCreate: true,
		LaunchConfig: LaunchConfig{
			ImageUuid:   "docker:" + image,
			Environment: env,
			Labels:      labels,
		},
	}

	// The transport error is checked before the status, which the request library
	// this replaced could not do: it returned a nil response together with the error
	// and every call site read the status off it first.
	resp, err := r.do(ctx, http.MethodPost, r.url+"services", reqBody)
	if err != nil {
		return id, fmt.Errorf("rancher API - could not create instance: %w", err), 0
	}
	code = resp.StatusCode
	if resp.StatusCode != http.StatusCreated {
		return id, errors.New("could not create instance: " + resp.Text()), code
	}

	data := map[string]interface{}{}
	err = resp.Decode(&data)
	if err != nil {
		return id, err, code
	}
	id, ok := data["id"].(string)
	if !ok {
		return id, errors.New("could not get service id"), code
	}
	return id, nil, code
}

func (r Rancher) RemoveContainer(ctx context.Context, id string) (err error) {
	resp, err := r.do(ctx, http.MethodDelete, r.url+"services/"+id, nil)
	if err != nil {
		return fmt.Errorf("rancher API - could not delete instance: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("unexpected status code while removing container " + strconv.Itoa(resp.StatusCode) + ": " + resp.Text())
	}
	return nil
}

func (r Rancher) UpdateContainer(ctx context.Context, id string, name string, image string, env map[string]string, restart bool, _ string, _ string, _ bool, bag map[string]string) (newId string, err error) {
	err = r.RemoveContainer(ctx, id)
	if err != nil {
		return newId, err
	}
	for {
		bytes, err := uuid.GenerateRandomBytes(64)
		if err != nil {
			return newId, err
		}
		rand := binary.BigEndian.Uint64(bytes)
		newId, err, code := r.createContainer(ctx, name+"-"+strconv.FormatUint(rand, 16), image, env, restart, bag)
		if err != nil {
			return newId, err
		}
		if code != http.StatusUnprocessableEntity {
			// if  code == http.StatusUnprocessableEntity probably reuse of old id
			return newId, err
		}
	}
}

func (r Rancher) ContainerExists(ctx context.Context, id string, _ *bool) (exists bool, err error) {
	resp, err := r.do(ctx, http.MethodGet, r.url+"services/"+id, nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return false, errors.New("unexpected status " + strconv.Itoa(resp.StatusCode))
	}
	return resp.StatusCode == http.StatusOK, nil
}

func (r Rancher) GetContainerStatus(_ context.Context, _ string, _ *bool) (status model.InstanceStatus, err error) {
	return status, deploy.ErrNotSupported
}

func (r Rancher) GetContainerStatuses(_ context.Context) (statuses map[string]model.InstanceStatus, err error) {
	return nil, deploy.ErrNotSupported
}

func (r Rancher) Disconnect() (err error) {
	return nil // not needed
}
