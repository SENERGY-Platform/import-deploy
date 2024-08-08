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

package database

import (
	"context"
	"log"
	"slices"
	"sync"

	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/database/mongo"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	permV2Client "github.com/SENERGY-Platform/permissions-v2/pkg/client"
	"golang.org/x/exp/maps"
)

func New(conf config.Config, ctx context.Context, wg *sync.WaitGroup) (db Database, err error) {
	perm := permV2Client.New(conf.PermissionV2Url)
	mong, err := mongo.New(perm, conf, ctx, wg)
	if err != nil {
		return db, err
	}
	db = mong
	err = migrate(mong, perm, ctx)
	if err != nil {
		return db, err
	}
	return
}

func migrate(db *mongo.Mongo, perm permV2Client.Client, ctx context.Context) error {
	log.Println("stat permv2 migration")
	perm.SetTopic(permV2Client.InternalAdminToken, permV2Client.Topic{
		Id:     model.PermV2InstanceTopic,
		NoCqrs: true,
	})

	instances, err := db.AdminListInstances(ctx, -1, 0, "", true, "", true)
	if err != nil {
		return err
	}

	permResources, err, _ := perm.ListResourcesWithAdminPermission(permV2Client.InternalAdminToken, model.PermV2InstanceTopic, permV2Client.ListOptions{})
	if err != nil {
		return err
	}
	permResouceMap := map[string]permV2Client.Resource{}
	for _, permResource := range permResources {
		permResouceMap[permResource.Id] = permResource
	}

	dbIds := []string{}
	for _, instance := range instances {
		dbIds = append(dbIds, instance.Id)

		permissions := permV2Client.ResourcePermissions{UserPermissions: map[string]permV2Client.PermissionsMap{}, GroupPermissions: map[string]permV2Client.PermissionsMap{}}
		resource, ok := permResouceMap[instance.Id]
		if ok {
			permissions.UserPermissions = resource.ResourcePermissions.UserPermissions
			permissions.GroupPermissions = resource.GroupPermissions
		}

		model.SetDefaultPermissions(instance, permissions)

		_, err, _ = perm.SetPermission(permV2Client.InternalAdminToken, model.PermV2InstanceTopic, instance.Id, permissions, permV2Client.SetPermissionOptions{})
		if err != nil {
			return err
		}
		log.Println(instance.Id, "migrated")
	}

	permResouceIds := maps.Keys(permResouceMap)

	for _, permResouceId := range permResouceIds {
		if !slices.Contains(dbIds, permResouceId) {
			err, _ = perm.RemoveResource(permV2Client.InternalAdminToken, model.PermV2InstanceTopic, permResouceId)
			if err != nil {
				return err
			}
			log.Println(permResouceId, "exists only in permv2, now deleted")
		}
	}
	return nil
}
