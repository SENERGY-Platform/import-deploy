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

package rancher2_api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"go.opentelemetry.io/otel"
	otelbaggage "go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

func init() {
	// baggage.AddLabels logs the entries it drops.
	log.InitForTest()
}

type call struct {
	method string
	path   string
	body   []byte
	header http.Header
}

// rancherStub answers with the status the test maps to a path, and records what it
// was asked. status defaults to 201 for POST and 204 for DELETE.
func rancherStub(t *testing.T, status map[string]int) (*Rancher2, *[]call) {
	t.Helper()
	var calls []call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, call{method: r.Method, path: r.URL.Path, body: body, header: r.Header.Clone()})
		if code, ok := status[r.URL.Path]; ok {
			w.WriteHeader(code)
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)
	return New(config.Config{
		RancherUrl:         server.URL + "/v3/",
		RancherAccessKey:   "key",
		RancherSecretKey:   "secret",
		RancherNamespaceId: "imports",
		RancherProjectId:   "c-abc:p-def",
	}), &calls
}

func contextWithBaggage(t *testing.T, entries map[string]string) context.Context {
	t.Helper()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))
	members := make([]otelbaggage.Member, 0, len(entries))
	for key, value := range entries {
		member, err := otelbaggage.NewMember(key, value)
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, member)
	}
	bag, err := otelbaggage.New(members...)
	if err != nil {
		t.Fatal(err)
	}
	return otelbaggage.ContextWithBaggage(context.Background(), bag)
}

func TestCreateContainer(t *testing.T) {
	rancher, calls := rancherStub(t, nil)
	ctx := contextWithBaggage(t, map[string]string{"smart_service_instance_id": "8fbd0e8a"})

	id, err := rancher.CreateContainer(ctx, "import-3c1f9b42", "repo/image:latest",
		map[string]string{"IMPORT_ID": "urn:infai:ses:import:3c1f9b42"}, true, "jonah",
		"urn:infai:ses:import-type:3c1f9b42", map[string]string{"smart_service_instance_id": "8fbd0e8a"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "import-3c1f9b42" {
		t.Errorf("expected the workload name as id, got %q", id)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected the workload and the vpa to be created, got %d calls", len(*calls))
	}

	workload := (*calls)[0]
	if workload.method != http.MethodPost || workload.path != "/v3/projects/c-abc:p-def/workloads" {
		t.Errorf("unexpected call %s %s", workload.method, workload.path)
	}
	// The standard library's parser through a throwaway request, so the test does
	// not re-implement base64 decoding.
	if u, p, hasAuth := (&http.Request{Header: workload.header}).BasicAuth(); !hasAuth || u != "key" || p != "secret" {
		t.Errorf("expected the rancher credentials, got %q", workload.header.Get("Authorization"))
	}

	// The point of the whole change: the caller's context goes onto the wire, and
	// the baggage becomes a pod label without touching the selector.
	if header := workload.header.Get("baggage"); header == "" {
		t.Error("expected the baggage to be propagated to the rancher API")
	}
	var request Request
	if err = json.Unmarshal(workload.body, &request); err != nil {
		t.Fatal(err)
	}
	label := baggage.LabelPrefix + "smart_service_instance_id"
	if request.Labels[label] != "8fbd0e8a" {
		t.Errorf("expected the baggage label on the workload, got %v", request.Labels)
	}
	if request.Containers[0].Labels[label] != "8fbd0e8a" {
		t.Errorf("expected the baggage label on the container, got %v", request.Containers[0].Labels)
	}
	if _, exists := request.Selector.MatchLabels[label]; exists {
		t.Errorf("the selector must stay free of the baggage, got %v", request.Selector.MatchLabels)
	}
	if request.Selector.MatchLabels["importId"] != "import-3c1f9b42" {
		t.Errorf("expected the plain labels in the selector, got %v", request.Selector.MatchLabels)
	}

	if (*calls)[1].path != "/k8s/clusters/c-abc/v1/autoscaling.k8s.io.verticalpodautoscalers" {
		t.Errorf("unexpected vpa path %q", (*calls)[1].path)
	}
}

// A rejected workload has to become an error rather than a started import, and the
// response body is what says why.
func TestCreateContainerOnAnErrorStatus(t *testing.T) {
	rancher, calls := rancherStub(t, map[string]int{
		"/v3/projects/c-abc:p-def/workloads": http.StatusUnprocessableEntity,
	})

	_, err := rancher.CreateContainer(context.Background(), "import-3c1f9b42", "repo/image:latest",
		nil, true, "jonah", "urn:infai:ses:import-type:3c1f9b42", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(*calls) != 1 {
		t.Errorf("the vpa must not be created after a failed workload, got %d calls", len(*calls))
	}
}

// An unreachable Rancher used to panic here: the request library returned a nil
// response together with the error and the status was read off it first.
func TestCreateContainerOnAnUnreachableRancher(t *testing.T) {
	rancher := New(config.Config{
		// Reserved by RFC 6761 for exactly this: guaranteed not to resolve.
		RancherUrl:       "http://something.invalid/v3/",
		RancherProjectId: "c-abc:p-def",
	})

	_, err := rancher.CreateContainer(context.Background(), "import-3c1f9b42", "repo/image:latest",
		nil, true, "jonah", "urn:infai:ses:import-type:3c1f9b42", nil)
	if err == nil {
		t.Fatal("expected an error for an unreachable rancher")
	}
}

// An import that does not restart is a job, and the workload path for it only shows
// up after the deployment path answered 404.
func TestRemoveContainerFallsBackToTheJob(t *testing.T) {
	rancher, calls := rancherStub(t, map[string]int{
		"/v3/projects/c-abc:p-def/workloads/deployment:imports:import-3c1f9b42": http.StatusNotFound,
	})

	if err := rancher.RemoveContainer(context.Background(), "import-3c1f9b42"); err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, c := range *calls {
		paths = append(paths, c.path)
	}
	want := []string{
		"/v3/projects/c-abc:p-def/workloads/deployment:imports:import-3c1f9b42",
		"/v3/projects/c-abc:p-def/workloads/job:imports:import-3c1f9b42",
		"/k8s/clusters/c-abc/v1/autoscaling.k8s.io.verticalpodautoscalers/imports/import-3c1f9b42-vpa",
		"/k8s/clusters/c-abc/v1/autoscaling.k8s.io.verticalpodautoscalercheckpoints/imports/import-3c1f9b42-vpa-import-3c1f9b42",
	}
	if len(paths) != len(want) {
		t.Fatalf("expected %v, got %v", want, paths)
	}
	for i, path := range want {
		if paths[i] != path {
			t.Errorf("expected %q, got %q", path, paths[i])
		}
	}
}

func TestContainerExistsFallsBackToTheJob(t *testing.T) {
	rancher, _ := rancherStub(t, map[string]int{
		"/v3/projects/c-abc:p-def/workloads/deployment:imports:import-3c1f9b42": http.StatusNotFound,
		"/v3/projects/c-abc:p-def/workloads/job:imports:import-3c1f9b42":        http.StatusOK,
	})

	exists, err := rancher.ContainerExists(context.Background(), "import-3c1f9b42", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected the job to count as existing")
	}
}
