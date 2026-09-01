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
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
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
