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
	"errors"
	"fmt"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"net/http"
	"strconv"
	"strings"

	"github.com/parnurzeal/gorequest"
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

func (r *Rancher2) UpdateContainer(id string, name string, image string, env map[string]string, restart bool, userid string, importTypeId string) (newId string, err error) {
	err = r.RemoveContainer(id)
	if err != nil {
		return newId, err
	}
	return r.CreateContainer(name, image, env, restart, userid, importTypeId)
}

func (r *Rancher2) CreateContainer(name string, image string, env map[string]string, restart bool, userid string, importTypeId string) (id string, err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	r2Env := []Env{}
	for k, v := range env {
		r2Env = append(r2Env, Env{
			Name:  k,
			Value: v,
		})
	}
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
			Labels: map[string]string{
				"user":         userid,
				"importId":     name,
				"importTypeId": importTypeId,
			},
		}},
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
	request.Method = "POST"
	request.Url = r.url + "projects/" + r.projectId
	if restart {
		request.Url += "/workloads"
		reqBody.Labels = map[string]string{"import": name}
		reqBody.Selector = Selector{MatchLabels: map[string]string{"import": name}}
		autoscaleRequestBody.Spec.TargetRef.ApiVersion = "apps/v1"
		autoscaleRequestBody.Spec.TargetRef.Kind = "Deployment"
	} else {
		request.Url += "/jobs"
		autoscaleRequestBody.Spec.TargetRef.ApiVersion = "batch/v1"
		autoscaleRequestBody.Spec.TargetRef.Kind = "Job"
	}
	resp, body, e := request.Send(reqBody).End()
	if resp.StatusCode != http.StatusCreated {
		err = errors.New("could not create import")
		fmt.Print(body)
		return
	}
	if len(e) > 0 {
		err = errors.New("could not create import")
		return
	}

	autoscaleRequest := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	resp, body, e = autoscaleRequest.Post(r.kubeUrl + "autoscaling.k8s.io.verticalpodautoscalers").Send(autoscaleRequestBody).End()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		err = errors.New("could not create import")
		fmt.Print(body)
		return
	}
	if len(e) > 0 {
		err = errors.New("could not create import")
		return
	}
	return name, err
}

func (r *Rancher2) RemoveContainer(id string) (err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	resp, body, e := request.Delete(r.url + "projects/" + r.projectId + "/workloads/deployment:" +
		r.namespaceId + ":" + id).End()
	if resp.StatusCode == http.StatusNotFound {
		resp, body, e = request.Delete(r.url + "projects/" + r.projectId + "/workloads/job:" +
			r.namespaceId + ":" + id).End()
	}
	if resp.StatusCode != http.StatusNoContent {
		err = errors.New("could not delete export: " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("something went wrong")
		return
	}

	autoscaleRequest := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	resp, body, e = autoscaleRequest.Delete(r.kubeUrl + "autoscaling.k8s.io.verticalpodautoscalers/" +
		r.namespaceId +
		"/" +
		id + "-vpa").
		End()
	if resp.StatusCode != http.StatusNoContent {
		err = errors.New("rancher2 API - could not delete operator vpa " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("something went wrong")
		return
	}

	autoscaleCheckpointRequest := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	resp, body, e = autoscaleCheckpointRequest.Delete(r.kubeUrl + "autoscaling.k8s.io.verticalpodautoscalercheckpoints/" +
		r.namespaceId +
		"/" +
		id + "-vpa-" + id).
		End()
	if resp.StatusCode != http.StatusNoContent {
		err = errors.New("rancher2 API - could not delete operator vpa checkpoint " + body)
		return
	}
	if len(e) > 0 {
		err = errors.New("something went wrong")
		return
	}

	return
}

func (r *Rancher2) ContainerExists(id string) (exists bool, err error) {
	request := gorequest.New().SetBasicAuth(r.accessKey, r.secretKey)
	resp, _, errs := request.Get(r.url + "projects/" + r.projectId + "/workloads/deployment:" +
		r.namespaceId + ":" + id).End()
	if len(errs) > 0 {
		return false, errs[0]
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return false, errors.New("unexpected status " + strconv.Itoa(resp.StatusCode))
	}
	if resp.StatusCode == http.StatusNotFound {
		resp, _, errs = request.Get(r.url + "projects/" + r.projectId + "/workloads/job:" +
			r.namespaceId + ":" + id).End()
		if len(errs) > 0 {
			return false, errs[0]
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return false, errors.New("unexpected status " + strconv.Itoa(resp.StatusCode))
		}
		return resp.StatusCode == http.StatusOK, nil
	}
	return true, nil
}

func (r *Rancher2) Disconnect() (err error) {
	return nil // not needed
}
