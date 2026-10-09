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
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// readableIds answers the permission listing per token, as permissions-v2 does per user.
type readableIds struct {
	permV2Client.Client
	byToken map[string][]string
	err     error
}

func (p readableIds) ListAccessibleResourceIdsContext(_ context.Context, token string, _ string, _ permV2Client.ListOptions, _ ...permV2Client.Permission) ([]string, error, int) {
	return p.byToken[token], p.err, 0
}

// usageDb needs a throwaway MongoDB without access control in MONGO_TEST_URL.
func usageDb(t *testing.T, perm permV2Client.Client) *Mongo {
	t.Helper()
	url := os.Getenv("MONGO_TEST_URL")
	if testing.Short() || url == "" {
		t.Skip("needs MONGO_TEST_URL, not in -short")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(url))
	if err != nil {
		t.Fatal(err)
	}
	dbName := "import_deploy_usage_test_" + randomHex(t)
	t.Cleanup(func() {
		_ = client.Database(dbName).Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	db := &Mongo{config: config.Config{MongoDatabase: dbName, MongoImportTypeCollection: "instances"}, client: client, perm: perm}
	for _, create := range CreateCollections {
		if err = create(db); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func insertInstances(t *testing.T, db *Mongo, instances ...model.Instance) {
	t.Helper()
	for _, instance := range instances {
		if _, err := db.instanceCollection().InsertOne(context.Background(), instance); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImportTypeUsage(t *testing.T) {
	const typeA, typeB = "urn:infai:ses:import-type:a", "urn:infai:ses:import-type:b"
	perm := readableIds{byToken: map[string][]string{
		"alice": {"i1", "i2", "i5"},
		"bob":   {"i3"},
	}}
	db := usageDb(t, perm)
	insertInstances(t, db,
		model.Instance{Id: "i1", Name: "zeta", ImportTypeId: typeA, Owner: "alice"},
		model.Instance{Id: "i2", Name: "alpha", ImportTypeId: typeA, Owner: "alice"},
		model.Instance{Id: "i3", Name: "bobs", ImportTypeId: typeA, Owner: "bob"},
		model.Instance{Id: "i4", Name: "carols", ImportTypeId: typeA, Owner: "carol", Generated: true},
		// readable to alice, but of another import type
		model.Instance{Id: "i5", Name: "other type", ImportTypeId: typeB, Owner: "alice"},
	)
	ctx := context.Background()

	t.Run("counts every user, lists what the caller reads", func(t *testing.T) {
		usage, err := db.ImportTypeUsage(ctx, jwt.Token{Token: "alice"}, typeA)
		if err != nil {
			t.Fatal(err)
		}
		want := model.ImportTypeUsage{Instances: 4, Readable: []model.InstanceRef{{Id: "i2", Name: "alpha"}, {Id: "i1", Name: "zeta"}}}
		if !reflect.DeepEqual(usage, want) {
			t.Errorf("usage = %+v, want %+v", usage, want)
		}
	})

	t.Run("another caller reads another part, the count stays", func(t *testing.T) {
		usage, err := db.ImportTypeUsage(ctx, jwt.Token{Token: "bob"}, typeA)
		if err != nil {
			t.Fatal(err)
		}
		want := model.ImportTypeUsage{Instances: 4, Readable: []model.InstanceRef{{Id: "i3", Name: "bobs"}}}
		if !reflect.DeepEqual(usage, want) {
			t.Errorf("usage = %+v, want %+v", usage, want)
		}
	})

	t.Run("a caller who reads none still sees the count", func(t *testing.T) {
		usage, err := db.ImportTypeUsage(ctx, jwt.Token{Token: "nobody"}, typeA)
		if err != nil {
			t.Fatal(err)
		}
		want := model.ImportTypeUsage{Instances: 4, Readable: []model.InstanceRef{}}
		if !reflect.DeepEqual(usage, want) {
			t.Errorf("usage = %+v, want %+v", usage, want)
		}
	})

	t.Run("unused import type", func(t *testing.T) {
		usage, err := db.ImportTypeUsage(ctx, jwt.Token{Token: "alice"}, "urn:infai:ses:import-type:unused")
		if err != nil {
			t.Fatal(err)
		}
		want := model.ImportTypeUsage{Instances: 0, Readable: []model.InstanceRef{}}
		if !reflect.DeepEqual(usage, want) {
			t.Errorf("usage = %+v, want %+v", usage, want)
		}
	})

	t.Run("the permission listing failing is an error", func(t *testing.T) {
		failing := &Mongo{config: db.config, client: db.client, perm: readableIds{err: errors.New("permissions down")}}
		if _, err := failing.ImportTypeUsage(ctx, jwt.Token{Token: "alice"}, typeA); err == nil {
			t.Error("expected an error, a count without the list would be a wrong answer")
		}
	})

	t.Run("the import type id is indexed", func(t *testing.T) {
		specs, err := db.instanceCollection().Indexes().ListSpecifications(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range specs {
			if spec.Name == "instanceImportTypeIdindex" {
				return
			}
		}
		t.Errorf("index instanceImportTypeIdindex missing")
	})
}
