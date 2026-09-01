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

package rancher2_api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/httpreq"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
)

type Rancher2 struct {
	url         string
	accessKey   string
	secretKey   string
	namespaceId string
	projectId   string
	kubeUrl     string
}

func New(config config.Config) *Rancher2 {
	kubeUrl := strings.TrimSuffix(config.RancherUrl, "v3/") + "k8s/clusters/" +
		strings.Split(config.RancherProjectId, ":")[0] + "/v1/"
	return &Rancher2{config.RancherUrl, config.RancherAccessKey, config.RancherSecretKey, config.RancherNamespaceId, config.RancherProjectId, kubeUrl}
}

// do sends a request to the Rancher API with the credentials, the trace and the
// baggage of ctx attached.
func (r *Rancher2) do(ctx context.Context, method string, url string, body any) (httpreq.Response, error) {
	return httpreq.Do(ctx, httpreq.Request{
		Method:    method,
		URL:       url,
		Body:      body,
		BasicAuth: &httpreq.BasicAuth{User: r.accessKey, Password: r.secretKey},
	})
}

func (r *Rancher2) UpdateContainer(ctx context.Context, id string, name string, image string, env map[string]string, restart bool, userid string, importTypeId string, _ bool, bag map[string]string) (newId string, err error) {
	err = r.RemoveContainer(ctx, id)
	if err != nil {
		return newId, err
	}
	return r.CreateContainer(ctx, name, image, env, restart, userid, importTypeId, bag)
}

func (r *Rancher2) CreateContainer(ctx context.Context, name string, image string, env map[string]string, restart bool, userid string, importTypeId string, bag map[string]string) (id string, err error) {
	r2Env := []Env{}
	for k, v := range env {
		r2Env = append(r2Env, Env{
			Name:  k,
			Value: v,
		})
	}
	labels := map[string]string{
		"user":         userid,
		"importId":     name,
		"importTypeId": strings.ReplaceAll(importTypeId, ":", "_"),
	}
	// The selector below keeps the plain labels: it is immutable once the workload
	// exists, while the baggage differs from one instance to the next.
	podLabels := baggage.AddLabels(ctx, maps.Clone(labels), bag)
	reqBody := &Request{
		Name:        name,
		NamespaceId: r.namespaceId,
		Containers: []Container{{
			Image:           image,
			Name:            name,
			Env:             r2Env,
			ImagePullPolicy: "Always",
			Resources: Resources{
				Requests: map[string]string{
					"memory": "128Mi",
					"cpu":    "100m",
				},
				Limits: map[string]string{
					"memory": "512Mi",
					"cpu":    "500m",
				},
			},
			Labels: podLabels,
		}},
		Labels:     podLabels,
		Scheduling: Scheduling{Scheduler: "default-scheduler", Node: Node{RequireAll: []string{"role=worker"}}},
	}

	autoscaleRequestBody := AutoscalingRequest{
		ApiVersion: "autoscaling.k8s.io/v1",
		Kind:       "VerticalPodAutoscaler",
		Metadata: AutoscalingRequestMetadata{
			Name:      name + "-vpa",
			Namespace: r.namespaceId,
		},
		Spec: AutoscalingRequestSpec{
			TargetRef: AutoscalingRequestTargetRef{
				Name: name,
			},
			UpdatePolicy: AutoscalingRequestUpdatePolicy{UpdateMode: "Auto"},
			ResourcePolicy: ResourcePolicy{
				ContainerPolicies: []ContainerPolicy{
					{
						ContainerName: "*",
						MaxAllowed: MaxAllowed{
							CPU:    1,
							Memory: "4000Mi",
						},
					},
				},
			},
		},
	}
	url := r.url + "projects/" + r.projectId
	if restart {
		url += "/workloads"
		reqBody.Selector = Selector{MatchLabels: labels}
		autoscaleRequestBody.Spec.TargetRef.ApiVersion = "apps/v1"
		autoscaleRequestBody.Spec.TargetRef.Kind = "Deployment"
	} else {
		url += "/jobs"
		autoscaleRequestBody.Spec.TargetRef.ApiVersion = "batch/v1"
		autoscaleRequestBody.Spec.TargetRef.Kind = "Job"
	}
	// The transport error is checked before the status, which the request library
	// this replaced could not do: it returned a nil response together with the error
	// and every call site read the status off it first.
	resp, err := r.do(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return id, fmt.Errorf("rancher2 API - could not create import: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		log.Logger.ErrorContext(ctx, "rancher2 API - could not create import", "status", resp.StatusCode, "response", resp.Text())
		return id, errors.New("could not create import")
	}

	resp, err = r.do(ctx, http.MethodPost, r.kubeUrl+"autoscaling.k8s.io.verticalpodautoscalers", autoscaleRequestBody)
	if err != nil {
		return id, fmt.Errorf("rancher2 API - could not create import vpa: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		log.Logger.ErrorContext(ctx, "rancher2 API - could not create import vpa", "status", resp.StatusCode, "response", resp.Text())
		return id, errors.New("could not create import")
	}
	return name, nil
}

func (r *Rancher2) RemoveContainer(ctx context.Context, id string) (err error) {
	resp, err := r.do(ctx, http.MethodDelete, r.url+"projects/"+r.projectId+"/workloads/deployment:"+
		r.namespaceId+":"+id, nil)
	if err != nil {
		return fmt.Errorf("rancher2 API - could not delete import: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		// An import that does not restart is a job rather than a deployment.
		resp, err = r.do(ctx, http.MethodDelete, r.url+"projects/"+r.projectId+"/workloads/job:"+
			r.namespaceId+":"+id, nil)
		if err != nil {
			return fmt.Errorf("rancher2 API - could not delete import: %w", err)
		}
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return errors.New("could not delete import: " + resp.Text())
	}

	resp, err = r.do(ctx, http.MethodDelete, r.kubeUrl+"autoscaling.k8s.io.verticalpodautoscalers/"+
		r.namespaceId+"/"+id+"-vpa", nil)
	if err != nil {
		return fmt.Errorf("rancher2 API - could not delete import vpa: %w", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return errors.New("rancher2 API - could not delete import vpa " + resp.Text())
	}

	resp, err = r.do(ctx, http.MethodDelete, r.kubeUrl+"autoscaling.k8s.io.verticalpodautoscalercheckpoints/"+
		r.namespaceId+"/"+id+"-vpa-"+id, nil)
	if err != nil {
		return fmt.Errorf("rancher2 API - could not delete import vpa checkpoint: %w", err)
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return errors.New("rancher2 API - could not delete import vpa checkpoint " + resp.Text())
	}

	return nil
}

func (r *Rancher2) ContainerExists(ctx context.Context, id string, _ *bool) (exists bool, err error) {
	resp, err := r.do(ctx, http.MethodGet, r.url+"projects/"+r.projectId+"/workloads/deployment:"+
		r.namespaceId+":"+id, nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return false, errors.New("unexpected status " + strconv.Itoa(resp.StatusCode))
	}
	if resp.StatusCode == http.StatusNotFound {
		resp, err = r.do(ctx, http.MethodGet, r.url+"projects/"+r.projectId+"/workloads/job:"+
			r.namespaceId+":"+id, nil)
		if err != nil {
			return false, err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return false, errors.New("unexpected status " + strconv.Itoa(resp.StatusCode))
		}
		return resp.StatusCode == http.StatusOK, nil
	}
	return true, nil
}

func (r *Rancher2) GetContainerStatus(_ context.Context, _ string, _ *bool) (status model.InstanceStatus, err error) {
	return status, deploy.ErrNotSupported
}

func (r *Rancher2) GetContainerStatuses(_ context.Context) (statuses map[string]model.InstanceStatus, err error) {
	return nil, deploy.ErrNotSupported
}

func (r *Rancher2) Disconnect() (err error) {
	return nil // not needed
}
