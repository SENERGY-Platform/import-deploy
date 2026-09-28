/*
 * Copyright 2020 InfAI (CC SES)
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

package mongo

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/SENERGY-Platform/go-service-base/struct-logger/attributes"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/log"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

type Mongo struct {
	config config.Config
	client *mongo.Client
	perm   permV2Client.Client
}

var CreateCollections = []func(db *Mongo) error{}

var (
	ErrEmptyDatabase   = errors.New("mongo database name must not be empty")
	ErrMissingPassword = errors.New("mongo password must not be empty when a mongo user is set")
)

const startupCheckTimeout = 10 * time.Second

func New(perm permV2Client.Client, conf config.Config, ctx context.Context, wg *sync.WaitGroup) (*Mongo, error) {
	if err := validateConfig(conf); err != nil {
		return nil, err
	}
	// The monitor gives every query a span under the trace of the request that
	// caused it, so a slow read shows up next to the handler that waited for it. It
	// needs an initialized OpenTelemetry, which lib.Start does before calling here.
	return start(ctx, wg, perm, conf, clientOptions(conf).SetMonitor(otelmongo.NewMonitor()), startupCheckTimeout)
}

// start disconnects the client on every failure path, so a failed startup leaves nothing connected.
func start(ctx context.Context, wg *sync.WaitGroup, perm permV2Client.Client, conf config.Config, opts *options.ClientOptions, timeout time.Duration) (*Mongo, error) {
	client, err := connect(ctx, opts, conf.MongoDatabase, timeout)
	if err != nil {
		return nil, err
	}
	db := &Mongo{config: conf, client: client, perm: perm}
	for _, creators := range CreateCollections {
		err = creators(db)
		if err != nil {
			db.Disconnect()
			return nil, err
		}
	}
	wg.Add(1)
	go func() {
		<-ctx.Done()
		_ = client.Disconnect(context.Background())
		wg.Done()
	}()
	return db, nil
}

// connect runs listCollections on the service database because Connect is lazy and ping needs no
// authentication; unreachable servers and wrong or missing credentials then fail at startup.
func connect(ctx context.Context, opts *options.ClientOptions, database string, timeout time.Duration) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	listOpts := options.ListCollections().SetNameOnly(true).SetAuthorizedCollections(true)
	if _, err = client.Database(database).ListCollectionNames(checkCtx, bson.D{}, listOpts); err != nil {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), timeout)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx)
		return nil, fmt.Errorf("mongo startup check failed: %w", err)
	}
	return client, nil
}

func validateConfig(conf config.Config) error {
	if conf.MongoDatabase == "" {
		return ErrEmptyDatabase
	}
	if conf.MongoUser != "" && conf.MongoPassword == "" {
		return ErrMissingPassword
	}
	return nil
}

// clientOptions applies the credentials after the URI so they replace any given in MONGO_URL.
func clientOptions(conf config.Config) *options.ClientOptions {
	opts := options.Client().ApplyURI(conf.MongoUrl)
	if conf.MongoUser != "" {
		opts.SetAuth(options.Credential{
			Username:   conf.MongoUser,
			Password:   conf.MongoPassword,
			AuthSource: conf.MongoAuthSource,
		})
	}
	return opts
}

func (this *Mongo) CreateId() string {
	return uuid.NewString()
}

func (this *Mongo) Transaction(ctx context.Context) (resultCtx context.Context, close func(success bool) error, err error) {
	if !this.config.MongoReplSet {
		return ctx, func(bool) error { return nil }, nil
	}
	session, err := this.client.StartSession()
	if err != nil {
		return nil, nil, err
	}
	err = session.StartTransaction()
	if err != nil {
		return nil, nil, err
	}

	//create session context; callback is executed synchronously and the error is passed on as error of WithSession
	_ = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		resultCtx = sessionContext
		return nil
	})

	return resultCtx, func(success bool) error {
		defer session.EndSession(context.Background())
		var err error
		if success {
			err = session.CommitTransaction(resultCtx)
		} else {
			err = session.AbortTransaction(resultCtx)
		}
		if err != nil {
			log.Logger.Error("unable to finish mongo transaction", attributes.ErrorKey, err)
		}
		return err
	}, nil
}

func (this *Mongo) ensureIndex(collection *mongo.Collection, indexname string, indexKey string, asc bool, unique bool) error {
	ctx, _ := getTimeoutContext()
	var direction int32 = -1
	if asc {
		direction = 1
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{indexKey, direction}},
		Options: options.Index().SetName(indexname).SetUnique(unique),
	})
	return err
}

func (this *Mongo) ensureCompoundIndex(collection *mongo.Collection, indexname string, asc bool, unique bool, indexKeys ...string) error {
	ctx, _ := getTimeoutContext()
	var direction int32 = -1
	if asc {
		direction = 1
	}
	keys := []bson.E{}
	for _, key := range indexKeys {
		keys = append(keys, bson.E{Key: key, Value: direction})
	}
	_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D(keys),
		Options: options.Index().SetName(indexname).SetUnique(unique),
	})
	return err
}

func (this *Mongo) Disconnect() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := this.client.Disconnect(ctx)
	if err != nil {
		log.Logger.Error("unable to disconnect mongo client", attributes.ErrorKey, err)
	}
}

func getBsonFieldName(obj interface{}, fieldName string) (bsonName string, err error) {
	field, found := reflect.TypeOf(obj).FieldByName(fieldName)
	if !found {
		return "", errors.New("field '" + fieldName + "' not found")
	}
	tags, err := bsoncodec.DefaultStructTagParser.ParseStructTags(field)
	if err != nil {
		return "", err
	}
	return tags.Name, err
}

func getTimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
