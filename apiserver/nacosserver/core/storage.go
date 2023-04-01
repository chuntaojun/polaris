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
	"time"

	nacosmodel "github.com/polarismesh/polaris/apiserver/nacosserver/model"
	"github.com/polarismesh/polaris/cache"
	"github.com/polarismesh/polaris/common/eventhub"
	"github.com/polarismesh/polaris/common/model"
)

type InstanceFilter func(ctx context.Context, svcInfo *nacosmodel.ServiceInfo,
	ins []*nacosmodel.Instance, healthyCount int32) *nacosmodel.ServiceInfo

func NewNacosDataStorage(ctx context.Context, cacheMgr *cache.CacheManager) *NacosDataStorage {

	subCtx, cancel := context.WithCancel(ctx)

	return &NacosDataStorage{
		cacheMgr:   cacheMgr,
		ctx:        subCtx,
		cancel:     cancel,
		namespaces: map[string]map[string]*ServiceData{},
	}
}

type NacosDataStorage struct {
	cacheMgr *cache.CacheManager

	ctx    context.Context
	cancel context.CancelFunc

	lock       sync.RWMutex
	namespaces map[string]map[string]*ServiceData

	revisions map[string]string
}

// ListInstances list nacos instances by filter
func (n *NacosDataStorage) ListInstances(ctx context.Context, svc model.ServiceKey,
	clusters []string, filter InstanceFilter) *nacosmodel.ServiceInfo {
	service := nacosmodel.GetServiceName(svc.Name)
	group := nacosmodel.GetServiceName(svc.Name)

	n.lock.RLock()
	defer n.lock.RUnlock()

	services, ok := n.namespaces[svc.Namespace]
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
	for i := range svcInfo.enableInstances {
		ins := svcInfo.enableInstances[i]
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

func (n *NacosDataStorage) RunSync() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.syncTask()
		}
	}
}

func (n *NacosDataStorage) syncTask() {
	// 定期将服务数据转为 Nacos 的服务数据缓存
	nsList := n.cacheMgr.Namespace().GetNamespaceList()
	needSync := map[string]*model.Service{}

	// 计算需要 refresh 的服务信息列表
	for _, ns := range nsList {
		_, svcs := n.cacheMgr.Service().ListServices(ns.Name)
		for _, svc := range svcs {
			revision := n.cacheMgr.GetServiceInstanceRevision(svc.ID)
			oldRevision, ok := n.revisions[svc.ID]
			if !ok || revision != oldRevision {
				needSync[svc.ID] = svc
			}
		}
	}

	svcInfos := make([]nacosmodel.SimpleServiceInfo, 0, len(needSync))

	// 遍历需要 refresh 数据的服务信息列表
	for _, svc := range needSync {
		svcData := n.loadNacosService(svc)
		svcInfos = append(svcInfos, nacosmodel.SimpleServiceInfo{
			Name:      svcData.name,
			GroupName: svcData.group,
		})
		instances := n.cacheMgr.Instance().GetInstancesByServiceID(svc.ID)
		svcData.loadInstances(instances)
	}

	// 发布服务信息变更事件
	eventhub.Publish(nacosmodel.NacosServicesChangeEventTopic, &nacosmodel.NacosServicesChangeEvent{
		Services: svcInfos,
	})
}

func (n *NacosDataStorage) loadNacosService(svc *model.Service) *ServiceData {
	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.namespaces[svc.Namespace]; !ok {
		n.namespaces[svc.Namespace] = map[string]*ServiceData{}
	}
	services := n.namespaces[svc.Namespace]
	if val, ok := services[svc.Name]; ok {
		return val
	}

	var (
		name  = svc.Name
		group = nacosmodel.DefaultServiceGroup
	)
	if val, ok := svc.Meta[nacosmodel.InternalNacosServiceType]; ok && val == "true" {
		name = nacosmodel.GetServiceName(svc.Name)
		group = nacosmodel.GetGroupName(svc.Name)
	}
	return &ServiceData{
		specService: svc,
		name:        name,
		group:       group,
		instances:   map[string]*nacosmodel.Instance{},
	}
}

type ServiceData struct {
	specService     *model.Service
	name            string
	group           string
	lock            sync.RWMutex
	enableInstances []*nacosmodel.Instance
	instances       map[string]*nacosmodel.Instance
}

func (s *ServiceData) loadInstances(instances []*model.Instance) {
	var (
		finalInstances       = map[string]*nacosmodel.Instance{}
		finalEnableInstances = make([]*nacosmodel.Instance, 0, 16)
	)

	for i := range instances {
		ins := &nacosmodel.Instance{}
		ins.FromSpecInstance(instances[i])
		if ins.Enabled {
			finalEnableInstances = append(finalEnableInstances, ins)
		}
		finalInstances[ins.Id] = ins
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.instances = finalInstances
	s.enableInstances = finalEnableInstances
}
