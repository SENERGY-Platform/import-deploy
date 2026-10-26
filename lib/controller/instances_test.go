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

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

// The environment variable is the complete context an import gets: the pod labels
// beside it are best-effort and drop what Kubernetes would refuse.
func TestGetEnvCarriesTheBaggage(t *testing.T) {
	control := &Controller{config: config.Config{KafkaBootstrap: "kafka:9092"}}
	instance := model.Instance{
		Id:         "urn:infai:ses:import:3c1f9b42",
		KafkaTopic: "urn_infai_ses_import_3c1f9b42",
		Baggage: map[string]string{
			"smart_service_instance_id": "8fbd0e8a",
			"username":                  "jonah@bitnify.net",
		},
	}

	env, err := control.getEnv(instance)
	if err != nil {
		t.Fatal(err)
	}
	want := "smart_service_instance_id=8fbd0e8a,username=jonah@bitnify.net"
	if env[baggage.EnvVar] != want {
		t.Errorf("expected %q, got %q", want, env[baggage.EnvVar])
	}
	if env["IMPORT_ID"] != instance.Id {
		t.Errorf("the existing environment must stay untouched, got %v", env)
	}
}

// Without baggage the variable is left out rather than set to an empty string: an
// import-lib that sees it empty would parse an empty context, and a container that
// never had one should look no different from before.
func TestGetEnvWithoutBaggage(t *testing.T) {
	control := &Controller{config: config.Config{KafkaBootstrap: "kafka:9092"}}

	env, err := control.getEnv(model.Instance{Id: "urn:infai:ses:import:3c1f9b42"})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := env[baggage.EnvVar]; exists {
		t.Errorf("expected no %s, got %q", baggage.EnvVar, env[baggage.EnvVar])
	}
}

// The owner is never part of a request body, so an update has to take it from the
// stored instance. Without it the deployment was written with an empty user label,
// which is part of its immutable selector, and Kubernetes refused the update; a
// container that had to be recreated instead lost its owner in the database as well.
func TestSetInstanceKeepsTheOwner(t *testing.T) {
	stored := storedInstance()
	control, db, deployment := updateController(t, stored)

	// What the web UI sends: the instance as it read it, edited and sent back.
	update := sentBack(t, stored)
	update.Name = "weather, renamed"

	err, code := control.SetInstance(context.Background(), update, jwt.Token{Token: "Bearer token"})
	if err != nil || code != http.StatusOK {
		t.Fatalf("expected the update to pass, got %v (%v)", err, code)
	}
	if deployment.userid != stored.Owner {
		t.Errorf("expected the deployment to keep the user %q, got %q", stored.Owner, deployment.userid)
	}
	if db.written == nil {
		t.Fatal("expected the instance to be written")
	}
	if db.written.Owner != stored.Owner {
		t.Errorf("expected the document to keep the owner %q, got %q", stored.Owner, db.written.Owner)
	}
	if db.written.Name != update.Name {
		t.Errorf("expected the update to be written, got the name %q", db.written.Name)
	}
}

// The creation time is part of the body a client reads and sends back, but it is the
// server's: a body that leaves it out must not zero it, and one that carries another
// value must not rewrite it.
func TestSetInstanceKeepsTheCreationTime(t *testing.T) {
	for _, test := range []struct {
		name      string
		createdAt time.Time
	}{
		{name: "a body without a creation time"},
		{name: "a body with another creation time", createdAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(test.name, func(t *testing.T) {
			stored := storedInstance()
			control, db, _ := updateController(t, stored)
			update := sentBack(t, stored)
			update.CreatedAt = test.createdAt

			err, code := control.SetInstance(context.Background(), update, jwt.Token{Token: "Bearer token"})
			if err != nil || code != http.StatusOK {
				t.Fatalf("expected the update to pass, got %v (%v)", err, code)
			}
			if db.written == nil {
				t.Fatal("expected the instance to be written")
			}
			if !db.written.CreatedAt.Equal(stored.CreatedAt) {
				t.Errorf("expected the creation time %v, got %v", stored.CreatedAt, db.written.CreatedAt)
			}
		})
	}
}

// storedInstance is a deployed instance as the database holds it.
func storedInstance() model.Instance {
	restart := true
	return model.Instance{
		Id:           "urn:infai:ses:import:3c1f9b42",
		Name:         "weather",
		ImportTypeId: "urn:infai:ses:import-type:8f2c0a51",
		Restart:      &restart,
		ServiceId:    "import-3c1f9b42",
		Owner:        "9a4c2e17",
		CreatedAt:    time.Date(2026, 3, 12, 9, 41, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 3, 12, 9, 41, 0, 0, time.UTC),
	}
}

// sentBack is an instance after a round trip through its api representation, the
// way a client reads it and sends it back.
func sentBack(t *testing.T, instance model.Instance) model.Instance {
	t.Helper()
	body, err := json.Marshal(instance)
	if err != nil {
		t.Fatal(err)
	}
	var result model.Instance
	if err = json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// updateController returns a controller that finds the given instance, may execute
// every import type and records what it deploys and writes.
func updateController(t *testing.T, stored model.Instance) (*Controller, *instanceStore, *recordingDeployment) {
	t.Helper()
	db := &instanceStore{stored: stored}
	deployment := &recordingDeployment{}
	control := importTypeController(t, func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(importTypeResponse))
	})
	control.db = db
	control.deploymentClient = deployment
	control.permv2 = grantAll{}
	return control, db, deployment
}

// instanceStore holds a single instance and records what is written back.
type instanceStore struct {
	Database
	stored  model.Instance
	written *model.Instance
}

func (this *instanceStore) GetInstance(_ context.Context, _ string, _ jwt.Token) (model.Instance, bool, error) {
	return this.stored, true, nil
}

func (this *instanceStore) SetInstance(_ context.Context, instance model.Instance, _ jwt.Token) error {
	this.written = &instance
	return nil
}

// recordingDeployment records the user a deployment is updated with.
type recordingDeployment struct {
	deploy.DeploymentClient
	userid string
}

func (this *recordingDeployment) UpdateContainer(_ context.Context, _ string, name string, _ string, _ map[string]string, _ bool, userid string, _ string, _ bool, _ map[string]string) (string, error) {
	this.userid = userid
	return name, nil
}

type grantAll struct {
	permV2Client.Client
}

func (grantAll) CheckPermissionContext(_ context.Context, _ string, _ string, _ string, _ ...permV2Client.Permission) (bool, error, int) {
	return true, nil, http.StatusOK
}
