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
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

const idFieldName = "Id"
const nameFieldName = "Name"
const ownerFieldName = "Owner"
const createdAtFieldName = "CreatedAt"
const updatedAtFieldName = "UpdatedAt"
const generatedFieldName = "Generated"
const imageFieldName = "Image"

var idKey string
var nameKey string
var ownerKey string
var createdAtKey string
var updatedAtKey string
var generatedKey string
var imageKey string

func init() {
	var err error
	idKey, err = getBsonFieldName(model.Instance{}, idFieldName)
	if err != nil {
		log.Fatal(err)
	}
	nameKey, err = getBsonFieldName(model.Instance{}, nameFieldName)
	if err != nil {
		log.Fatal(err)
	}
	ownerKey, err = getBsonFieldName(model.Instance{}, ownerFieldName)
	if err != nil {
		log.Fatal(err)
	}
	createdAtKey, err = getBsonFieldName(model.Instance{}, createdAtFieldName)
	if err != nil {
		log.Fatal(err)
	}
	updatedAtKey, err = getBsonFieldName(model.Instance{}, updatedAtFieldName)
	if err != nil {
		log.Fatal(err)
	}
	generatedKey, err = getBsonFieldName(model.Instance{}, generatedFieldName)
	if err != nil {
		log.Fatal(err)
	}
	imageKey, err = getBsonFieldName(model.Instance{}, imageFieldName)
	if err != nil {
		log.Fatal(err)
	}

	CreateCollections = append(CreateCollections, func(db *Mongo) error {
		collection := db.client.Database(db.config.MongoTable).Collection(db.config.MongoImportTypeCollection)
		err = db.ensureCompoundIndex(collection, "instanceOwnerIdindex", true, true, ownerKey, idKey)
		if err != nil {
			return err
		}
		return nil
	})
}

func (this *Mongo) instanceCollection() *mongo.Collection {
	return this.client.Database(this.config.MongoTable).Collection(this.config.MongoImportTypeCollection)
}

func (this *Mongo) GetInstance(ctx context.Context, id string, owner string) (instance model.Instance, exists bool, err error) {
	result := this.instanceCollection().FindOne(ctx, bson.M{ownerKey: owner, idKey: id})
	err = result.Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return instance, false, errors.New("requested instance nonexistent")
		}
		return instance, false, err
	}
	err = result.Decode(&instance)
	if err == mongo.ErrNoDocuments {
		return instance, false, nil
	}
	for idx, config := range instance.Configs {
		err = configToRead(&config)
		if err != nil {
			return instance, true, err
		}
		instance.Configs[idx] = config
	}
	return instance, true, err
}

func (this *Mongo) ListInstances(ctx context.Context, limit int64, offset int64, sort string, owner string, asc bool, search string, includeGenerated bool) (result []model.Instance, err error) {
	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	sortby := idKey
	switch sort {
	case "id":
		sortby = idKey
	case "name":
		sortby = nameKey
	case "created_at":
		sortby = createdAtKey
	case "updated_at":
		sortby = updatedAtKey
	case "image":
		sortby = imageKey
	default:
		sortby = idKey
	}
	direction := int32(1)
	if !asc {
		direction = int32(-1)
	}
	opt.SetSort(bson.D{{sortby, direction}})
	var filter bson.M
	if includeGenerated {
		filter = bson.M{nameKey: primitive.Regex{
			Pattern: ".*" + search + ".*",
		}}
	} else {
		// filter for generatedKey == False || generatedKey == undefined to find legacy instances
		filter = bson.M{"$or": []bson.M{{generatedKey: false}, {generatedKey: bson.M{"$exists": false}}},
			nameKey: primitive.Regex{
				Pattern: ".*" + search + ".*",
			}}
	}
	if owner != "" {
		filter[ownerKey] = owner
	}
	cursor, err := this.instanceCollection().Find(ctx, filter, opt)
	if err != nil {
		return nil, err
	}
	for cursor.Next(context.Background()) {
		instance := model.Instance{}
		err = cursor.Decode(&instance)
		if err != nil {
			return nil, err
		}
		for idx, config := range instance.Configs {
			err = configToRead(&config)
			if err != nil {
				return result, err
			}
			instance.Configs[idx] = config
		}
		result = append(result, instance)
	}
	err = cursor.Err()
	return
}

func (this *Mongo) SetInstance(ctx context.Context, instance model.Instance, owner string) error {
	for idx, conf := range instance.Configs {
		err := configToWrite(&conf)
		if err != nil {
			return err
		}
		instance.Configs[idx] = conf
	}
	_, err := this.instanceCollection().ReplaceOne(ctx, bson.M{ownerKey: owner, idKey: instance.Id}, instance, options.Replace().SetUpsert(true))
	return err
}

func (this *Mongo) RemoveInstance(ctx context.Context, id string, owner string) error {
	_, err := this.instanceCollection().DeleteOne(ctx, bson.M{ownerKey: owner, idKey: id})
	return err
}

func configToWrite(config *model.InstanceConfig) error {
	if config == nil {
		return errors.New("nil config")
	}
	_, valid := config.Value.(map[string]interface{})
	if !valid {
		return nil
	}

	bs, err := json.Marshal(config.Value)
	if err != nil {
		return err
	}
	s := string(bs)
	config.ValueString = &s
	config.Value = nil
	return nil
}

func configToRead(config *model.InstanceConfig) error {
	if config == nil {
		return errors.New("nil config")
	}
	if config.ValueString == nil {
		return nil
	}
	config.Value = map[string]interface{}{}
	err := json.Unmarshal([]byte(*config.ValueString), &config.Value)
	if err != nil {
		return err
	}
	config.ValueString = nil
	return nil
}
