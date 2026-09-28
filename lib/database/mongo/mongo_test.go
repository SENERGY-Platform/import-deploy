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
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMain(m *testing.M) {
	log.InitForTest()
	os.Exit(m.Run())
}

const replicaSetURL = "mongodb://mongo-0.mongo:27017,mongo-1.mongo:27017/?replicaSet=rs0&readPreference=primary"

func TestClientOptions_AuthWhenUserGiven(t *testing.T) {
	opts := clientOptions(config.Config{
		MongoUrl:        replicaSetURL,
		MongoUser:       "import-deploy",
		MongoPassword:   "s3cr3t",
		MongoAuthSource: "admin",
		MongoDatabase:   "import_deploy",
	})
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	want := &options.Credential{Username: "import-deploy", Password: "s3cr3t", AuthSource: "admin"}
	if !reflect.DeepEqual(opts.Auth, want) {
		t.Errorf("auth = %+v, want %+v", opts.Auth, want)
	}
}

func TestClientOptions_NoAuthWhenUserEmpty(t *testing.T) {
	// A password without a user must not switch auth on.
	opts := clientOptions(config.Config{
		MongoUrl:        "mongodb://localhost:27017",
		MongoPassword:   "s3cr3t",
		MongoAuthSource: "admin",
		MongoDatabase:   "import_deploy",
	})
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	if opts.Auth != nil {
		t.Errorf("auth = %+v, want nil", opts.Auth)
	}
}

func TestClientOptions_ConfiguredCredentialsReplaceURICredentials(t *testing.T) {
	opts := clientOptions(config.Config{
		MongoUrl:        "mongodb://old:oldpw@localhost:27017/?authSource=other&authMechanism=SCRAM-SHA-1",
		MongoUser:       "import-deploy",
		MongoPassword:   "newpw",
		MongoAuthSource: "admin",
		MongoDatabase:   "import_deploy",
	})
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	want := &options.Credential{Username: "import-deploy", Password: "newpw", AuthSource: "admin"}
	if !reflect.DeepEqual(opts.Auth, want) {
		t.Errorf("auth = %+v, want %+v", opts.Auth, want)
	}
}

func TestClientOptions_URIPassedUnchanged(t *testing.T) {
	opts := clientOptions(config.Config{MongoUrl: replicaSetURL, MongoDatabase: "import_deploy"})
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := opts.GetURI(); got != replicaSetURL {
		t.Errorf("uri = %q, want %q", got, replicaSetURL)
	}
	if want := []string{"mongo-0.mongo:27017", "mongo-1.mongo:27017"}; !reflect.DeepEqual(opts.Hosts, want) {
		t.Errorf("hosts = %v, want %v", opts.Hosts, want)
	}
	if opts.ReplicaSet == nil || *opts.ReplicaSet != "rs0" {
		t.Errorf("replica set = %v, want rs0", opts.ReplicaSet)
	}
}

func TestClientOptions_NoSchemeAdded(t *testing.T) {
	opts := clientOptions(config.Config{MongoUrl: "localhost:27017", MongoDatabase: "import_deploy"})
	if err := opts.Validate(); err == nil {
		t.Fatal("expected an error for a url without scheme")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr error
	}{
		{"no auth", config.Config{MongoDatabase: "import_deploy"}, nil},
		{"user and password", config.Config{MongoDatabase: "import_deploy", MongoUser: "u", MongoPassword: "p"}, nil},
		{"password without user", config.Config{MongoDatabase: "import_deploy", MongoPassword: "p"}, nil},
		{"user without password", config.Config{MongoDatabase: "import_deploy", MongoUser: "u"}, ErrMissingPassword},
		{"empty database", config.Config{}, ErrEmptyDatabase},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateConfig(tt.cfg); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstanceCollectionUsesConfiguredDatabase(t *testing.T) {
	// mongo.Connect does not contact the server, so no running instance is needed.
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	db := &Mongo{config: config.Config{MongoDatabase: "custom_db", MongoImportTypeCollection: "instances"}, client: client}
	coll := db.instanceCollection()
	if coll.Database().Name() != "custom_db" || coll.Name() != "instances" {
		t.Errorf("collection = %s.%s, want custom_db.instances", coll.Database().Name(), coll.Name())
	}
}

// unreachableURL points at a port that was just free, so only the startup check can fail.
func unreachableURL(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	return "mongodb://" + addr + "/?directConnection=true"
}

// The startup check would fail as well, so these check for the specific validation error.
func TestNew_RejectsBeforeConnecting(t *testing.T) {
	cases := map[string]config.Config{
		"empty database":        {MongoUser: "import-deploy", MongoPassword: "s3cr3t"},
		"user without password": {MongoUser: "import-deploy", MongoDatabase: "import_deploy"},
	}
	want := map[string]error{"empty database": ErrEmptyDatabase, "user without password": ErrMissingPassword}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			cfg.MongoUrl = unreachableURL(t)
			wg := &sync.WaitGroup{}
			db, err := New(nil, cfg, context.Background(), wg)
			if !errors.Is(err, want[name]) {
				t.Fatalf("err = %v, want %v", err, want[name])
			}
			if db != nil {
				t.Error("expected no db on failure")
			}
			if strings.Contains(err.Error(), "s3cr3t") {
				t.Errorf("error leaks the password: %v", err)
			}
		})
	}
}

// poolCounter counts connection pools; Disconnect closes every pool Connect created.
type poolCounter struct{ created, closed int32 }

func (p *poolCounter) monitor() *event.PoolMonitor {
	return &event.PoolMonitor{Event: func(e *event.PoolEvent) {
		switch e.Type {
		case event.PoolCreated:
			p.created++
		case event.PoolClosedEvent:
			p.closed++
		}
	}}
}

func (p *poolCounter) assertAllClosed(t *testing.T) {
	t.Helper()
	if p.created == 0 || p.closed != p.created {
		t.Errorf("%d of %d connection pools closed, the client was left connected", p.closed, p.created)
	}
}

func assertNoWaiters(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("a failed startup left a shutdown routine registered")
	}
}

func TestStart_StartupCheckFailsWithoutServer(t *testing.T) {
	const password = "pw-must-not-appear-7f3a"
	conf := config.Config{
		MongoUrl:                  unreachableURL(t),
		MongoUser:                 "import-deploy",
		MongoPassword:             password,
		MongoAuthSource:           "admin",
		MongoDatabase:             "import_deploy",
		MongoImportTypeCollection: "instances",
	}
	pools := &poolCounter{}
	wg := &sync.WaitGroup{}
	begin := time.Now()
	db, err := start(context.Background(), wg, nil, conf, clientOptions(conf).SetPoolMonitor(pools.monitor()), 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected an error when the server is unreachable")
	}
	if db != nil {
		t.Error("expected no db on failure")
	}
	if !strings.HasPrefix(err.Error(), "mongo startup check failed: ") {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), password) {
		t.Error("error text contains the password")
	}
	if elapsed := time.Since(begin); elapsed > 5*time.Second {
		t.Errorf("start took %v, the timeout was not applied", elapsed)
	}
	pools.assertAllClosed(t)
	assertNoWaiters(t, wg)
}
