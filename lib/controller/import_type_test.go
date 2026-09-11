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
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// importTypeResponse is an import type as the import repository answers it. Both
// aspect fields are filled there: AspectIds is the one to read, AspectId is kept in
// sync for clients that predate the list.
const importTypeResponse = `{
	"id": "urn:infai:ses:import-type:8f2c0a51",
	"name": "weather",
	"description": "reads a weather api",
	"image": "ghcr.io/senergy-platform/import-weather:latest",
	"default_restart": true,
	"configs": [
		{"name": "interval", "description": "seconds between two reads", "type": "https://schema.org/Integer", "default_value": 300},
		{"name": "location", "description": "", "type": "https://schema.org/StructuredValue", "default_value": {"lat": 51.34, "long": 12.37}}
	],
	"output": {
		"name": "root",
		"type": "https://schema.org/StructuredValue",
		"characteristic_id": "",
		"sub_content_variables": [
			{
				"name": "temperature",
				"type": "https://schema.org/Float",
				"characteristic_id": "urn:infai:ses:characteristic:degree-celsius",
				"function_id": "urn:infai:ses:measuring-function:temperature",
				"aspect_id": "urn:infai:ses:aspect:air",
				"aspect_ids": ["urn:infai:ses:aspect:air", "urn:infai:ses:aspect:outside"]
			}
		]
	},
	"owner": "jonah",
	"cost": 42
}`

// The import type is decoded into the shared model, which carries the output with its
// aspects. The local copy it replaced had no output field at all, so everything below
// the configs was dropped on decode -- a reader that follows an import back to the
// aspects of its data would have found nothing here.
func TestGetImportTypeKeepsTheOutputAspects(t *testing.T) {
	control := importTypeController(t, func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(importTypeResponse))
	})

	importType, err, code := control.getImportType(context.Background(), "urn:infai:ses:import-type:8f2c0a51", jwt.Token{Token: "Bearer token"})
	if err != nil || code != http.StatusOK {
		t.Fatalf("expected the import type, got %v (%v)", err, code)
	}

	if len(importType.Output.SubContentVariables) != 1 {
		t.Fatalf("expected the output to survive the decode, got %#v", importType.Output)
	}
	temperature := importType.Output.SubContentVariables[0]
	want := []string{"urn:infai:ses:aspect:air", "urn:infai:ses:aspect:outside"}
	if !slices.Equal(temperature.AspectIds, want) {
		t.Errorf("expected the aspect ids %v, got %v", want, temperature.AspectIds)
	}
	// The deprecated single aspect is kept beside the list rather than folded away, so
	// that this service hands on what the import repository sent.
	if temperature.AspectId != "urn:infai:ses:aspect:air" {
		t.Errorf("expected the deprecated aspect id to stay, got %q", temperature.AspectId)
	}
	if temperature.FunctionId != "urn:infai:ses:measuring-function:temperature" {
		t.Errorf("expected the function id, got %q", temperature.FunctionId)
	}
	if importType.Cost != 42 {
		t.Errorf("expected the cost, got %v", importType.Cost)
	}
}

// What the deployment itself runs on has to come through unchanged: the consolidation
// is a type change, not a behavior change.
func TestGetImportTypeKeepsTheDeploymentFields(t *testing.T) {
	control := importTypeController(t, func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(importTypeResponse))
	})

	importType, err, _ := control.getImportType(context.Background(), "urn:infai:ses:import-type:8f2c0a51", jwt.Token{Token: "Bearer token"})
	if err != nil {
		t.Fatal(err)
	}
	if importType.Image != "ghcr.io/senergy-platform/import-weather:latest" {
		t.Errorf("expected the image, got %q", importType.Image)
	}
	if !importType.DefaultRestart {
		t.Error("expected default_restart to be true")
	}
	if len(importType.Configs) != 2 {
		t.Fatalf("expected both configs, got %#v", importType.Configs)
	}
	if importType.Configs[0].Type != model.Integer || importType.Configs[0].DefaultValue != float64(300) {
		t.Errorf("expected the integer config, got %#v", importType.Configs[0])
	}
	// A structure default value reaches a client as the object it is; the string form
	// the shared type carries beside it is the import repository's storage detail and
	// is never sent.
	if _, isObject := importType.Configs[1].DefaultValue.(map[string]interface{}); !isObject {
		t.Errorf("expected the structure config to stay an object, got %#v", importType.Configs[1].DefaultValue)
	}
	if importType.Configs[1].DefaultValueString != nil {
		t.Errorf("expected no serialized default value, got %q", *importType.Configs[1].DefaultValueString)
	}
}

// The request is built here rather than taken from the import repository's client,
// because that client knows no context: the caller's trace and baggage would not
// reach the import repository, and neither would the timeout.
func TestGetImportTypeCarriesTheCallersContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	var received http.Header
	control := importTypeController(t, func(writer http.ResponseWriter, request *http.Request) {
		received = request.Header.Clone()
		_, _ = writer.Write([]byte(importTypeResponse))
	})

	ctx, err := baggage.WithValue(context.Background(), baggage.ImportIdKey, "urn_infai_ses_import_3c1f9b42")
	if err != nil {
		t.Fatal(err)
	}
	_, err, _ = control.getImportType(ctx, "urn:infai:ses:import-type:8f2c0a51", jwt.Token{Token: "Bearer token"})
	if err != nil {
		t.Fatal(err)
	}

	if received.Get("Authorization") != "Bearer token" {
		t.Errorf("expected the caller's token, got %q", received.Get("Authorization"))
	}
	if received.Get("Baggage") != "import_id=urn_infai_ses_import_3c1f9b42" {
		t.Errorf("expected the caller's baggage, got %q", received.Get("Baggage"))
	}
}

// An import type that cannot be read is not an internal error: the status the import
// repository answered with is what the caller gets back.
func TestGetImportTypeAnswers(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		message string
	}{
		{name: "an unknown import type", status: http.StatusNotFound, message: "unknown import type"},
		{name: "an import type the caller may not read", status: http.StatusForbidden, message: "no access to import type"},
		{name: "an import repository that is unwell", status: http.StatusInternalServerError, message: "unexpected status code"},
	} {
		t.Run(test.name, func(t *testing.T) {
			control := importTypeController(t, func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
			})

			_, err, code := control.getImportType(context.Background(), "urn:infai:ses:import-type:8f2c0a51", jwt.Token{Token: "Bearer token"})
			if code != test.status {
				t.Errorf("expected the status %v, got %v", test.status, code)
			}
			if err == nil || err.Error() != test.message {
				t.Errorf("expected %q, got %v", test.message, err)
			}
		})
	}
}

// importTypeController returns a controller pointed at an import repository that
// answers with the given handler.
func importTypeController(t *testing.T, handler http.HandlerFunc) *Controller {
	t.Helper()
	log.InitForTest()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Controller{config: config.Config{ImportRepoUrl: server.URL}}
}

// The config values of an instance are validated against the type constants of the
// shared model. They are the schema.org urls the import repository writes into an
// import type, and a constant that drifted from them would let every value through or
// refuse every one, without anything failing to compile.
func TestValidateConfigAgainstTheSharedTypes(t *testing.T) {
	for _, test := range []struct {
		name  string
		conf  model.ImportTypeConfig
		value interface{}
		valid bool
	}{
		{name: "a text value", conf: model.ImportTypeConfig{Name: "url", Type: model.String}, value: "https://example.org", valid: true},
		{name: "a number where text is declared", conf: model.ImportTypeConfig{Name: "url", Type: model.String}, value: float64(3), valid: false},
		{name: "a whole number for an integer", conf: model.ImportTypeConfig{Name: "interval", Type: model.Integer}, value: float64(300), valid: true},
		{name: "a fraction for an integer", conf: model.ImportTypeConfig{Name: "interval", Type: model.Integer}, value: 1.5, valid: false},
		{name: "a fraction for a float", conf: model.ImportTypeConfig{Name: "factor", Type: model.Float}, value: 1.5, valid: true},
		{name: "a boolean", conf: model.ImportTypeConfig{Name: "verbose", Type: model.Boolean}, value: true, valid: true},
		{name: "a list", conf: model.ImportTypeConfig{Name: "topics", Type: model.List}, value: []interface{}{"a", "b"}, valid: true},
		{name: "a structure", conf: model.ImportTypeConfig{Name: "location", Type: model.Structure}, value: map[string]interface{}{"lat": 51.34}, valid: true},
		{name: "an unset value", conf: model.ImportTypeConfig{Name: "interval", Type: model.Integer}, value: nil, valid: true},
		{name: "a config without a name", conf: model.ImportTypeConfig{Type: model.String}, value: "https://example.org", valid: false},
		{name: "a type outside the shared model", conf: model.ImportTypeConfig{Name: "url", Type: model.Type("https://schema.org/Thing")}, value: "https://example.org", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if validateConfig(test.conf, test.value) != test.valid {
				t.Errorf("expected valid=%v for %#v", test.valid, test.value)
			}
		})
	}
}
