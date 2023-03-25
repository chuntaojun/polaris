/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package model

import (
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type ServiceInfo struct {
	Name                     string      `json:"name"`
	GroupName                string      `json:"groupName"`
	Clusters                 string      `json:"clusters"`
	Hosts                    []*Instance `json:"hosts"`
	Checksum                 string      `json:"checksum"`
	CacheMillis              int64       `json:"cacheMillis"`
	LastRefTime              int64       `json:"lastRefTime"`
	ReachProtectionThreshold bool        `json:"reachProtectionThreshold"`
}

func NewEmptyServiceInfo(name, group string) *ServiceInfo {
	return &ServiceInfo{
		Name:      name,
		GroupName: group,
	}
}

type Instance struct {
	Id          string            `json:"instanceId"`
	IP          string            `json:"ip"`
	Port        int32             `json:"port"`
	Weight      float64           `json:"weight"`
	Healthy     bool              `json:"healthy"`
	Enabled     bool              `json:"enabled"`
	Ephemeral   bool              `json:"ephemeral"`
	ClusterName string            `json:"clusterName"`
	ServiceName string            `json:"serviceName"`
	Metadata    map[string]string `json:"metadata"`
}

func (i *Instance) DeepClone() *Instance {
	copyMeta := make(map[string]string, len(i.Metadata))
	for k, v := range i.Metadata {
		copyMeta[k] = v
	}

	return &Instance{
		Id:          i.Id,
		IP:          i.IP,
		Port:        i.Port,
		Weight:      i.Weight,
		Healthy:     i.Healthy,
		Enabled:     i.Enabled,
		Ephemeral:   i.Ephemeral,
		ClusterName: i.ClusterName,
		ServiceName: i.ServiceName,
		Metadata:    copyMeta,
	}
}

func (i *Instance) ToSpecInstance() *apiservice.Instance {
	return &apiservice.Instance{
		Id: &wrapperspb.StringValue{
			Value: i.Id,
		},
		Service: &wrapperspb.StringValue{
			Value: i.ServiceName,
		},
		Host: &wrapperspb.StringValue{
			Value: i.IP,
		},
		Port: &wrapperspb.UInt32Value{
			Value: uint32(i.Port),
		},
		Weight: &wrapperspb.UInt32Value{
			Value: uint32(i.Weight),
		},
		EnableHealthCheck: &wrapperspb.BoolValue{
			Value: true,
		},
		HealthCheck: &apiservice.HealthCheck{
			Type: apiservice.HealthCheck_HEARTBEAT,
			Heartbeat: &apiservice.HeartbeatHealthCheck{
				Ttl: &wrapperspb.UInt32Value{
					Value: 5,
				},
			},
		},
		Healthy: &wrapperspb.BoolValue{
			Value: i.Healthy,
		},
		Isolate: &wrapperspb.BoolValue{
			Value: !i.Enabled,
		},
		Metadata: i.Metadata,
	}
}

type ClientBeat struct {
	Namespace   string            `json:"namespace"`
	ServiceName string            `json:"serviceName"`
	Cluster     string            `json:"cluster"`
	Ip          string            `json:"ip"`
	Port        int               `json:"port"`
	Weight      float64           `json:"weight"`
	Ephemeral   bool              `json:"ephemeral"`
	Metadata    map[string]string `json:"metadata"`
}

func (c *ClientBeat) ToSpecInstance() (*apiservice.Instance, error) {
	return nil, nil
}
