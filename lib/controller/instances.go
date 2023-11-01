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

package controller

import (
	"encoding/json"
	"errors"
	"github.com/SENERGY-Platform/import-deploy/lib/auth"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/import-deploy/lib/util"
	"github.com/hashicorp/go-uuid"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

const idPrefix = "urn:infai:ses:import:"
const containerNamePrefix = "import-"

func (this *Controller) ListInstances(jwt auth.Token, limit int64, offset int64, sort string, asc bool, search string, includeGenerated bool) (results []model.Instance, err error, errCode int) {
	ctx, _ := util.GetTimeoutContext()
	results, err = this.db.ListInstances(ctx, limit, offset, sort, jwt.GetUserId(), asc, search, includeGenerated)
	if err != nil {
		return results, err, http.StatusInternalServerError
	}
	return results, nil, http.StatusOK
}

func (this *Controller) CountInstances(jwt auth.Token, search string, includeGenerated bool) (count int64, err error, errCode int) {
	ctx, _ := util.GetTimeoutContext()
	count, err = this.db.CountInstances(ctx, jwt.GetUserId(), search, includeGenerated)
	if err != nil {
		return count, err, http.StatusInternalServerError
	}
	return count, nil, http.StatusOK
}

func (this *Controller) ReadInstance(id string, jwt auth.Token) (result model.Instance, err error, errCode int) {
	ctx, _ := util.GetTimeoutContext()
	result, exists, err := this.db.GetInstance(ctx, id, jwt.GetUserId())
	if !exists {
		return result, err, http.StatusNotFound
	}
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return result, nil, http.StatusOK
}

func (this *Controller) CreateInstance(instance model.Instance, jwt auth.Token) (result model.Instance, err error, code int) {
	if instance.Id != "" {
		return result, errors.New("explicit setting of id not allowed"), http.StatusBadRequest
	}
	if instance.KafkaTopic != "" {
		return result, errors.New("explicit setting of kafka topic not allowed"), http.StatusBadRequest
	}
	id, err := uuid.GenerateUUID()
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	instance.Id = idPrefix + id
	instance.Owner = jwt.GetUserId()
	instance, err, code = this.fillDefaultValues(instance, jwt)
	if err != nil || code != http.StatusOK {
		return result, err, code
	}

	access, err := this.hasXAccess(jwt, instance.ImportTypeId)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	if !access {
		return result, errors.New("no execute access to importType"), http.StatusForbidden
	}

	env, err := this.getEnv(instance)
	if err != nil {
		return result, err, http.StatusBadRequest
	}
	err = this.kafkaAdmin.CreateTopic(instance.KafkaTopic)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	var restart bool
	if instance.Restart == nil || *instance.Restart {
		restart = true
	} else {
		restart = false
	}
	instance.ServiceId, err = this.deploymentClient.CreateContainer(containerNamePrefix+strings.TrimPrefix(instance.Id, idPrefix), instance.Image, env, restart)
	if err != nil {
		return result, err, http.StatusInternalServerError
	}

	now := time.Now()
	instance.CreatedAt = now
	instance.UpdatedAt = now
	ctx, _ := util.GetTimeoutContext()
	err = this.db.SetInstance(ctx, instance, jwt.GetUserId())
	if err != nil {
		return result, err, http.StatusInternalServerError
	}
	return instance, nil, http.StatusOK
}

func (this *Controller) SetInstance(instance model.Instance, jwt auth.Token) (err error, code int) {
	ctx, _ := util.GetTimeoutContext()
	existing, exists, err := this.db.GetInstance(ctx, instance.Id, jwt.GetUserId())
	if !exists {
		return errors.New("not found"), http.StatusNotFound
	}
	if err != nil {
		return err, http.StatusInternalServerError
	}
	instance.Owner = jwt.GetUserId()
	if existing.ImportTypeId != instance.ImportTypeId {
		return errors.New("change of import type not supported"), http.StatusBadRequest
	}
	instance, err, code = this.fillDefaultValues(instance, jwt)
	if err != nil || code != http.StatusOK {
		return err, code
	}

	access, err := this.hasXAccess(jwt, instance.ImportTypeId)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	if !access {
		return errors.New("no execute access to importType"), http.StatusForbidden
	}

	env, err := this.getEnv(instance)
	if err != nil {
		return err, http.StatusBadRequest
	}
	var restart bool
	if instance.Restart == nil || *instance.Restart {
		restart = true
	} else {
		restart = false
	}

	instance.ServiceId, err = this.deploymentClient.UpdateContainer(existing.ServiceId, containerNamePrefix+strings.TrimPrefix(instance.Id, idPrefix), instance.Image, env, restart)
	if err != nil {
		return err, http.StatusInternalServerError
	}
	instance.UpdatedAt = time.Now()
	ctx, _ = util.GetTimeoutContext()
	err = this.db.SetInstance(ctx, instance, jwt.GetUserId())
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusOK
}

func (this *Controller) DeleteInstance(id string, jwt auth.Token) (err error, errCode int) {
	ctx, _ := util.GetTimeoutContext()
	instance, exists, err := this.db.GetInstance(ctx, id, jwt.GetUserId())
	if !exists {
		return errors.New("not found"), http.StatusNotFound
	}
	if err != nil {
		return err, http.StatusInternalServerError
	}
	err = this.deploymentClient.RemoveContainer(instance.ServiceId)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	err = this.kafkaAdmin.DeleteTopic(instance.KafkaTopic)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	err = this.db.RemoveInstance(ctx, id, jwt.GetUserId())
	if err != nil {
		return err, http.StatusInternalServerError
	}
	return nil, http.StatusNoContent
}

func (this *Controller) EnsureAllInstancesDeployed() (err error) {
	var offset int64 = 0
	var batchSize int64 = 100
	for {
		ctx, _ := util.GetTimeoutContext()
		instances, err := this.db.ListInstances(ctx, batchSize, offset, "name", "", true, "", true)
		if err != nil {
			return err
		}
		if len(instances) == 0 {
			return nil // done
		}
		offset += int64(len(instances))
		for _, instance := range instances {
			exists, err := this.deploymentClient.ContainerExists(instance.ServiceId)
			if err != nil {
				return err
			}
			if exists {
				log.Println(instance.Id + " still exists")
				continue
			}
			log.Println("Recreating " + instance.Id)
			env, err := this.getEnv(instance)
			if err != nil {
				return err
			}
			var restart bool
			if instance.Restart == nil || *instance.Restart {
				restart = true
			} else {
				restart = false
			}
			instance.ServiceId, err = this.deploymentClient.CreateContainer(containerNamePrefix+strings.TrimPrefix(instance.Id, idPrefix), instance.Image, env, restart)
			if err != nil {
				return err
			}
			ctx, _ := util.GetTimeoutContext()
			err = this.db.SetInstance(ctx, instance, instance.Owner)
			if err != nil {
				return err
			}
		}
	}
}

func (this *Controller) fillDefaultValues(instance model.Instance, jwt auth.Token) (result model.Instance, err error, code int) {
	importType, err, code := this.getImportType(instance.ImportTypeId, jwt)
	if err != nil {
		return instance, err, code
	}
	if len(instance.Image) > 0 && instance.Image != importType.Image {
		return instance, errors.New("imageType uses different image"), http.StatusBadRequest
	}
	if len(instance.Image) == 0 {
		instance.Image = importType.Image
	}
	for _, typeConf := range importType.Configs {
		idx, ok := indexOf(instance.Configs, typeConf.Name)
		if !ok {
			instance.Configs = append(instance.Configs, model.InstanceConfig{
				Name:  typeConf.Name,
				Value: typeConf.DefaultValue,
			})
		}
		if (ok && !validateConfig(typeConf, instance.Configs[idx].Value)) || (!ok && !validateConfig(typeConf, instance.Configs[len(instance.Configs)-1].Value)) {
			return instance, errors.New("config value of wrong type"), http.StatusBadRequest
		}
	}
	if instance.Restart == nil {
		instance.Restart = &importType.DefaultRestart
	}
	instance.KafkaTopic = strings.ReplaceAll(instance.Id, ":", "_")
	return instance, nil, http.StatusOK
}

func (this *Controller) getImportType(id string, jwt auth.Token) (importType model.ImportType, err error, code int) {
	req, err := http.NewRequest("GET", this.config.ImportRepoUrl+"/import-types/"+id, nil)
	req.Header.Set("Authorization", jwt.Token)
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return importType, errors.New("unable to contact import repo"), http.StatusBadGateway
	}
	if resp.StatusCode == http.StatusNotFound {
		return importType, errors.New("unknown import type"), resp.StatusCode
	}
	if resp.StatusCode == http.StatusForbidden {
		return importType, errors.New("no access to import type"), resp.StatusCode
	}
	if resp.StatusCode != http.StatusOK {
		return importType, errors.New("unexpected status code"), resp.StatusCode
	}
	err = json.NewDecoder(resp.Body).Decode(&importType)
	return importType, err, resp.StatusCode
}

func indexOf(list []model.InstanceConfig, element string) (int, bool) {
	for idx, c := range list {
		if c.Name == element {
			return idx, true
		}
	}
	return -1, false
}

func validateConfig(conf model.ImportTypeConfig, val interface{}) (valid bool) {
	valid = true
	if len(conf.Name) == 0 ||
		(conf.Type != model.String &&
			conf.Type != model.Integer &&
			conf.Type != model.Float &&
			conf.Type != model.List &&
			conf.Type != model.Structure &&
			conf.Type != model.Boolean) {
		return false
	}
	if val != nil {
		switch conf.Type {
		case model.String:
			_, valid = val.(string)
			break
		case model.Integer:
			val, validInner := val.(float64)
			valid = validInner && math.Mod(val, 1) == 0
			break
		case model.Float:
			_, valid = val.(float64)
			break
		case model.List:
			_, valid = val.([]interface{})
			break
		case model.Structure:
			_, valid = val.(interface{})
			break
		case model.Boolean:
			_, valid = val.(bool)
			break
		}
	}
	return valid
}

func (this *Controller) getEnv(instance model.Instance) (m map[string]string, err error) {
	m = map[string]string{}
	confJson := map[string]interface{}{}
	for _, conf := range instance.Configs {
		confJson[conf.Name] = conf.Value
	}
	confBytes, err := json.Marshal(confJson)
	if err != nil {
		return m, err
	}
	m["CONFIG"] = string(confBytes)
	m["KAFKA_TOPIC"] = instance.KafkaTopic
	m["KAFKA_BOOTSTRAP"] = this.config.KafkaBootstrap
	m["IMPORT_ID"] = instance.Id
	return m, nil
}

func (this *Controller) hasXAccess(jwt auth.Token, importTypeId string) (bool, error) {
	return this.checkBool(jwt, "import-types", importTypeId, model.EXECUTE)
}
