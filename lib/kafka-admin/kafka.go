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

package kafkaAdmin

import (
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/Shopify/sarama"
)

type KafkaAdminImpl struct {
	config config.Config
	admin  sarama.ClusterAdmin
}

func New(config config.Config) (*KafkaAdminImpl, error) {
	sconfig := sarama.NewConfig()
	sconfig.Version = sarama.V2_4_0_0
	admin, err := sarama.NewClusterAdmin([]string{config.KafkaBootstrap}, sconfig)
	if err != nil {
		return nil, err
	}
	return &KafkaAdminImpl{
		config: config,
		admin:  admin,
	}, nil
}

func (this *KafkaAdminImpl) CreateTopic(name string) (err error) {
	minus1 := "-1"
	topicConfig := map[string]*string{}
	topicConfig["retention.bytes"] = &minus1
	topicConfig["retention.ms"] = &minus1
	detail := sarama.TopicDetail{
		NumPartitions:     1,
		ReplicationFactor: 1,
		ConfigEntries:     topicConfig,
	}
	return this.admin.CreateTopic(name, &detail, false)
}

func (this *KafkaAdminImpl) DeleteTopic(name string) (err error) {
	return this.admin.DeleteTopic(name)
}

func (this *KafkaAdminImpl) Disconnect() (err error) {
	return this.admin.Close()
}
