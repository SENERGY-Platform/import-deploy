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
	"log"
	"net/http"

	"github.com/SENERGY-Platform/import-deploy/lib/model"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"github.com/SENERGY-Platform/service-commons/pkg/jwt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func (this *Mongo) GetInstance(ctx context.Context, id string, jwt jwt.Token) (instance model.Instance, exists bool, err error) {
	ok, err, _ := this.perm.CheckPermission(jwt.Token, model.PermV2InstanceTopic, id, permV2Client.Read)
	if err != nil {
		return instance, false, err
	}
	if !ok {
		return instance, false, errors.New("requested instance nonexistent")
	}
	result := this.instanceCollection().FindOne(ctx, bson.M{idKey: id})
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

func (this *Mongo) AdminListInstances(ctx context.Context, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool) (result []model.Instance, err error) {
	return this.listInstances(ctx, limit, offset, sort, asc, search, includeGenerated, []string{}, true)
}

func (this *Mongo) ListInstances(ctx context.Context, limit int64, offset int64, sort string, jwt jwt.Token, asc bool, search string, includeGenerated bool) (result []model.Instance, err error) {
	ids, err, _ := this.perm.ListAccessibleResourceIds(jwt.Token, model.PermV2InstanceTopic, permV2Client.ListOptions{}, permV2Client.Read)
	if err != nil {
		return nil, err
	}
	return this.listInstances(ctx, limit, offset, sort, asc, search, includeGenerated, ids, false)
}

func (this *Mongo) listInstances(ctx context.Context, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool, ids []string, ignoreIdFilter bool) (result []model.Instance, err error) {
	opt := options.Find()
	if limit != -1 {
		opt.SetLimit(limit)
	}
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
	if !ignoreIdFilter {
		filter[idKey] = bson.M{"$in": ids}
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

func (this *Mongo) SetInstance(ctx context.Context, instance model.Instance, jwt jwt.Token) error {
	for idx, conf := range instance.Configs {
		err := configToWrite(&conf)
		if err != nil {
			return err
		}
		instance.Configs[idx] = conf
	}
	_, err := this.instanceCollection().ReplaceOne(ctx, bson.M{ownerKey: instance.Owner, idKey: instance.Id}, instance, options.Replace().SetUpsert(true))
	if err != nil {
		return err
	}
	permissions := permV2Client.ResourcePermissions{
		GroupPermissions: map[string]permV2Client.PermissionsMap{},
		UserPermissions:  map[string]permV2Client.PermissionsMap{},
	}
	permResource, err, code := this.perm.GetResource(permV2Client.InternalAdminToken, model.PermV2InstanceTopic, instance.Id)
	if err != nil && code != http.StatusNotFound {
		return err
	}
	if code == http.StatusOK {
		permissions.GroupPermissions = permResource.GroupPermissions
		permissions.UserPermissions = permResource.UserPermissions
	}
	model.SetDefaultPermissions(instance, permissions)
	_, err, _ = this.perm.SetPermission(permV2Client.InternalAdminToken, model.PermV2InstanceTopic, instance.Id, permissions, permV2Client.SetPermissionOptions{})
	return err
}

func (this *Mongo) RemoveInstance(ctx context.Context, id string, jwt jwt.Token) error {
	ok, err, _ := this.perm.CheckPermission(jwt.Token, model.PermV2InstanceTopic, id, permV2Client.Administrate)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("requested instance nonexistent or missing rights")
	}
	_, err = this.instanceCollection().DeleteOne(ctx, bson.M{idKey: id})
	return err
}

func (this *Mongo) CountInstances(ctx context.Context, jwt jwt.Token, search string, includeGenerated bool) (int64, error) {
	ids, err, _ := this.perm.ListAccessibleResourceIds(jwt.Token, model.PermV2InstanceTopic, permV2Client.ListOptions{}, permV2Client.Read)
	if err != nil {
		return 0, err
	}
	filter := bson.D{{Key: idKey, Value: bson.M{"$in": ids}}}
	if includeGenerated {
		filter = append(filter, primitive.E{Key: nameKey, Value: primitive.Regex{
			Pattern: ".*" + search + ".*",
		}})
	} else {
		// filter for generatedKey == False || generatedKey == undefined to find legacy instances
		filter = append(filter, primitive.E{
			Key: "$or",
			Value: []bson.M{
				{generatedKey: false},
				{generatedKey: bson.M{"$exists": false}},
			},
		})

		filter = append(filter, primitive.E{
			Key: nameKey,
			Value: primitive.Regex{
				Pattern: ".*" + search + ".*",
			},
		})
	}
	count, err := this.instanceCollection().CountDocuments(ctx, filter)
	return count, err
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
