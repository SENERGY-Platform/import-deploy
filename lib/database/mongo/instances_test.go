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

package mongo

import (
	"context"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
)

func TestListAndCountByImportType(t *testing.T) {
	const typeA, typeB = "urn:infai:ses:import-type:a", "urn:infai:ses:import-type:b"
	perm := readableIds{byToken: map[string][]string{"alice": {"i1", "i2", "i3", "i5"}}}
	db := usageDb(t, perm)
	insertInstances(t, db,
		model.Instance{Id: "i1", Name: "zeta", ImportTypeId: typeA, Owner: "alice"},
		model.Instance{Id: "i2", Name: "alpha", ImportTypeId: typeA, Owner: "alice"},
		model.Instance{Id: "i3", Name: "generated", ImportTypeId: typeA, Owner: "alice", Generated: true},
		// of the type, but not readable to alice
		model.Instance{Id: "i4", Name: "bobs", ImportTypeId: typeA, Owner: "bob"},
		model.Instance{Id: "i5", Name: "other type", ImportTypeId: typeB, Owner: "alice"},
	)
	ctx := context.Background()
	alice := jwt.Token{Token: "alice"}

	cases := []struct {
		name             string
		importTypeId     string
		includeGenerated bool
		search           string
		want             []string
	}{
		{"one type, readable only", typeA, true, "", []string{"i2", "i3", "i1"}},
		{"with the generated filter", typeA, false, "", []string{"i2", "i1"}},
		{"with a search", typeA, true, "eta", []string{"i1"}},
		{"another type", typeB, true, "", []string{"i5"}},
		{"no type narrows nothing", "", true, "", []string{"i2", "i3", "i5", "i1"}},
		{"unused type", "urn:infai:ses:import-type:unused", true, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			instances, err := db.ListInstances(ctx, 100, 0, "name", alice, true, c.search, c.includeGenerated, nil, c.importTypeId)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, instance := range instances {
				got = append(got, instance.Id)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("listed %v, want %v", got, c.want)
			}
			count, err := db.CountInstances(ctx, alice, c.search, c.includeGenerated, c.importTypeId)
			if err != nil {
				t.Fatal(err)
			}
			if count != int64(len(c.want)) {
				t.Errorf("counted %d, want %d", count, len(c.want))
			}
		})
	}
}
