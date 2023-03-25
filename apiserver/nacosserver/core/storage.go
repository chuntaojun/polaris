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

package core

import (
	"context"
	"strings"
	"sync"

	nacosmodel "github.com/polarismesh/polaris/apiserver/nacosserver/model"
	"github.com/polarismesh/polaris/common/model"
)

type InstanceFilter func(ctx context.Context, svcInfo *nacosmodel.ServiceInfo, ins []*nacosmodel.Instance, healthyCount int32) *nacosmodel.ServiceInfo

type NacosDataStorage struct {
	lock     sync.RWMutex
	services map[string]map[string]*ServiceData
}

type ServiceData struct {
	lock      sync.RWMutex
	instances map[string]*nacosmodel.Instance
}

func (n *NacosDataStorage) ListInstances(ctx context.Context, svc model.ServiceKey, clusters []string, filter InstanceFilter) *nacosmodel.ServiceInfo {
	service := nacosmodel.GetServiceName(svc.Name)
	group := nacosmodel.GetServiceName(svc.Name)

	n.lock.RLock()
	defer n.lock.RUnlock()

	services, ok := n.services[svc.Namespace]
	if !ok {
		return nacosmodel.NewEmptyServiceInfo(service, group)
	}
	svcInfo, ok := services[svc.Name]
	if !ok {
		return nacosmodel.NewEmptyServiceInfo(service, group)
	}

	clusterSet := make(map[string]struct{})
	for i := range clusters {
		clusterSet[clusters[i]] = struct{}{}
	}

	ret := make([]*nacosmodel.Instance, 0, 32)

	svcInfo.lock.RLock()
	defer svcInfo.lock.RUnlock()

	resultInfo := &nacosmodel.ServiceInfo{
		CacheMillis:              1000,
		Name:                     service,
		GroupName:                group,
		Clusters:                 strings.Join(clusters, ","),
		ReachProtectionThreshold: false,
	}

	healthCount := int32(0)
	for i := range svcInfo.instances {
		ins := svcInfo.instances[i]
		if !ins.Enabled {
			continue
		}
		if _, ok := clusterSet[ins.ClusterName]; !ok {
			continue
		}
		if ins.Healthy {
			healthCount++
		}
		ret = append(ret, ins)
	}

	if filter == nil {
		resultInfo.Hosts = ret
		return resultInfo
	}
	return filter(ctx, resultInfo, ret, healthCount)
}
