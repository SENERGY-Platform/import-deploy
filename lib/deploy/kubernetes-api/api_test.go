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

package kubernetes_api

import (
	"context"
	"maps"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	// baggage.AddLabels logs the entries it drops.
	log.InitForTest()
}

// A deployment's selector is immutable, so the baggage -- which differs from one
// instance to the next and did not exist for the ones already deployed -- must stay
// out of it. The log aggregation reads pod labels, so that is where it belongs.
func TestTheBaggageStaysOutOfTheSelector(t *testing.T) {
	labels := map[string]string{
		"user":         "jonah",
		"importId":     "import-3c1f9b42",
		"importTypeId": "urn_infai_ses_import-type_3c1f9b42",
	}
	podLabels := baggage.AddLabels(context.Background(), maps.Clone(labels),
		map[string]string{"smart_service_instance_id": "8fbd0e8a"})

	deployment := getDeployment("import-3c1f9b42", labels, podLabels, corev1.Container{}, nil)

	if !maps.Equal(labels, deployment.Spec.Selector.MatchLabels) {
		t.Errorf("expected the plain labels in the selector, got %v", deployment.Spec.Selector.MatchLabels)
	}
	got := deployment.Spec.Template.ObjectMeta.Labels
	if got[baggage.LabelPrefix+"smart_service_instance_id"] != "8fbd0e8a" {
		t.Errorf("expected the baggage on the pod template, got %v", got)
	}
	for key, value := range labels {
		if got[key] != value {
			t.Errorf("the pod template must keep the plain labels too, got %v", got)
		}
	}
}
