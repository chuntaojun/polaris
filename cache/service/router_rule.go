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

package service

import (
	"fmt"
	"sort"
	"sync"
	"time"

	apitraffic "github.com/polarismesh/specification/source/go/api/v1/traffic_manage"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	types "github.com/polarismesh/polaris/cache/api"
	"github.com/polarismesh/polaris/common/model"
	"github.com/polarismesh/polaris/common/utils"
	"github.com/polarismesh/polaris/store"
)

type (
	// routingConfigCache Routing rules cache
	routingConfigCache struct {
		*types.BaseCache

		serviceCache types.ServiceCache
		storage      store.Store

		bucket *routeRuleBucket

		lastMtimeV1 time.Time
		lastMtimeV2 time.Time

		singleFlight singleflight.Group

		// pendingV1RuleIds Records need to be converted from V1 to V2 routing rules ID
		plock            sync.Mutex
		pendingV1RuleIds map[string]*model.RoutingConfig
	}
)

// NewRoutingConfigCache Return a object of operating RoutingConfigcache
func NewRoutingConfigCache(s store.Store, cacheMgr types.CacheManager) types.RoutingConfigCache {
	return &routingConfigCache{
		BaseCache: types.NewBaseCache(s, cacheMgr),
		storage:   s,
	}
}

// initialize The function of implementing the cache interface
func (rc *routingConfigCache) Initialize(_ map[string]interface{}) error {
	rc.lastMtimeV1 = time.Unix(0, 0)
	rc.lastMtimeV2 = time.Unix(0, 0)
	rc.pendingV1RuleIds = make(map[string]*model.RoutingConfig)
	rc.bucket = newRouteRuleBucket()
	rc.serviceCache = rc.BaseCache.CacheMgr.GetCacher(types.CacheService).(*serviceCache)
	return nil
}

// Update The function of implementing the cache interface
func (rc *routingConfigCache) Update() error {
	// Multiple thread competition, only one thread is updated
	_, err, _ := rc.singleFlight.Do(rc.Name(), func() (interface{}, error) {
		return nil, rc.DoCacheUpdate(rc.Name(), rc.realUpdate)
	})
	return err
}

// update The function of implementing the cache interface
func (rc *routingConfigCache) realUpdate() (map[string]time.Time, int64, error) {
	outV1, err := rc.storage.GetRoutingConfigsForCache(rc.LastFetchTime(), rc.IsFirstUpdate())
	if err != nil {
		log.Errorf("[Cache] routing config v1 cache get from store err: %s", err.Error())
		return nil, -1, err
	}

	outV2, err := rc.storage.GetRoutingConfigsV2ForCache(rc.LastFetchTime(), rc.IsFirstUpdate())
	if err != nil {
		log.Errorf("[Cache] routing config v2 cache get from store err: %s", err.Error())
		return nil, -1, err
	}

	lastMtimes := map[string]time.Time{}
	rc.setRoutingConfigV1(lastMtimes, outV1)
	rc.setRoutingConfigV2(lastMtimes, outV2)
	return lastMtimes, int64(len(outV1) + len(outV2)), err
}

// Clear The function of implementing the cache interface
func (rc *routingConfigCache) Clear() error {
	rc.BaseCache.Clear()
	rc.pendingV1RuleIds = make(map[string]*model.RoutingConfig)
	rc.bucket = newRouteRuleBucket()
	rc.lastMtimeV1 = time.Unix(0, 0)
	rc.lastMtimeV2 = time.Unix(0, 0)
	return nil
}

// Name The function of implementing the cache interface
func (rc *routingConfigCache) Name() string {
	return types.RoutingConfigName
}

func (rc *routingConfigCache) ListRouterRule(service, namespace string) []*model.ExtendRouterConfig {
	routerRules := rc.bucket.listEnableRules(service, namespace, true)
	ret := make([]*model.ExtendRouterConfig, 0, len(routerRules))
	for level := range routerRules {
		items := routerRules[level]
		ret = append(ret, items...)
	}
	return ret
}

// GetRouterConfigV2 Obtain routing configuration based on serviceid
func (rc *routingConfigCache) GetRouterConfigV2(id, service, namespace string) (*apitraffic.Routing, error) {
	if id == "" && service == "" && namespace == "" {
		return nil, nil
	}

	routerRules := rc.bucket.listEnableRules(service, namespace, true)
	revisions := make([]string, 0, 8)
	rulesV2 := make([]*apitraffic.RouteRule, 0, len(routerRules))
	for level := range routerRules {
		items := routerRules[level]
		for i := range items {
			entry, err := items[i].ToApi()
			if err != nil {
				return nil, err
			}
			rulesV2 = append(rulesV2, entry)
			revisions = append(revisions, entry.GetRevision())
		}
	}
	revision, err := types.CompositeComputeRevision(revisions)
	if err != nil {
		log.Warn("[Cache][Routing] v2=>v1 compute revisions fail, use fake revision", zap.Error(err))
		revision = utils.NewV2Revision()
	}

	resp := &apitraffic.Routing{
		Namespace: utils.NewStringValue(namespace),
		Service:   utils.NewStringValue(service),
		Rules:     rulesV2,
		Revision:  utils.NewStringValue(revision),
	}
	return resp, nil
}

// GetRouterConfig Obtain routing configuration based on serviceid
func (rc *routingConfigCache) GetRouterConfig(id, service, namespace string) (*apitraffic.Routing, error) {
	if id == "" && service == "" && namespace == "" {
		return nil, nil
	}

	routerRules := rc.bucket.listEnableRules(service, namespace, false)
	inBounds, outBounds, revisions := rc.convertV2toV1(routerRules, service, namespace)
	revision, err := types.CompositeComputeRevision(revisions)
	if err != nil {
		log.Warn("[Cache][Routing] v2=>v1 compute revisions fail, use fake revision", zap.Error(err))
		revision = utils.NewV2Revision()
	}

	resp := &apitraffic.Routing{
		Namespace: utils.NewStringValue(namespace),
		Service:   utils.NewStringValue(service),
		Inbounds:  inBounds,
		Outbounds: outBounds,
		Revision:  utils.NewStringValue(revision),
	}

	return formatRoutingResponseV1(resp), nil
}

// formatRoutingResponseV1 Give the client's cache, no need to expose EXTENDINFO information data
func formatRoutingResponseV1(ret *apitraffic.Routing) *apitraffic.Routing {
	inBounds := ret.Inbounds
	outBounds := ret.Outbounds

	for i := range inBounds {
		inBounds[i].ExtendInfo = nil
	}

	for i := range outBounds {
		outBounds[i].ExtendInfo = nil
	}
	return ret
}

// IteratorRouterRule
func (rc *routingConfigCache) IteratorRouterRule(iterProc types.RouterRuleIterProc) {
	// need to traverse the Routing cache bucket of V2 here
	rc.bucket.foreach(iterProc)
}

// GetRoutingConfigCount Get the total number of routing configuration cache
func (rc *routingConfigCache) GetRoutingConfigCount() int {
	return rc.bucket.size()
}

// setRoutingConfigV1 Update the data of the store to the cache and convert to v2 model
func (rc *routingConfigCache) setRoutingConfigV1(lastMtimes map[string]time.Time, cs []*model.RoutingConfig) {
	rc.plock.Lock()
	defer rc.plock.Unlock()

	if len(cs) == 0 {
		return
	}
	lastMtimeV1 := rc.LastMtime(rc.Name()).Unix()
	for _, entry := range cs {
		if entry.ID == "" {
			continue
		}
		if entry.ModifyTime.Unix() > lastMtimeV1 {
			lastMtimeV1 = entry.ModifyTime.Unix()
		}
		if !entry.Valid {
			// Delete the cache converted to V2
			rc.bucket.deleteV1(entry.ID)
			continue
		}
		rc.pendingV1RuleIds[entry.ID] = entry
	}

	for id := range rc.pendingV1RuleIds {
		entry := rc.pendingV1RuleIds[id]
		// Save to the new V2 cache
		ok, v2rule, err := rc.convertV1toV2(entry)
		if err != nil {
			log.Warn("[Cache] routing parse v1 => v2 fail, will try again next",
				zap.String("rule-id", entry.ID), zap.Error(err))
			continue
		}
		if !ok {
			log.Warn("[Cache] routing parse v1 => v2 is nil, will try again next",
				zap.String("rule-id", entry.ID))
			continue
		}
		if ok && v2rule != nil {
			delete(rc.pendingV1RuleIds, id)
			rc.bucket.saveV1(entry, v2rule)
		}
	}

	lastMtimes[rc.Name()] = time.Unix(lastMtimeV1, 0)
	log.Infof("[Cache] convert routing parse v1 => v2 count : %d", rc.bucket.convertV2Size())
}

// setRoutingConfigV2 Store V2 Router Caches
func (rc *routingConfigCache) setRoutingConfigV2(lastMtimes map[string]time.Time, cs []*model.RouterConfig) {
	if len(cs) == 0 {
		return
	}

	lastMtimeV2 := rc.LastMtime(rc.Name() + "v2").Unix()
	for _, entry := range cs {
		if entry.ID == "" {
			continue
		}
		if entry.ModifyTime.Unix() > lastMtimeV2 {
			lastMtimeV2 = entry.ModifyTime.Unix()
		}
		if !entry.Valid {
			rc.bucket.deleteV2(entry.ID)
			continue
		}
		extendEntry, err := entry.ToExpendRoutingConfig()
		if err != nil {
			log.Error("[Cache] routing config v2 convert to expend", zap.Error(err))
			continue
		}
		rc.bucket.saveV2(extendEntry)
	}
	lastMtimes[rc.Name()+"v2"] = time.Unix(lastMtimeV2, 0)
}

func (rc *routingConfigCache) IsConvertFromV1(id string) (string, bool) {
	val, ok := rc.bucket.v1rulesToOld[id]
	return val, ok
}

func (rc *routingConfigCache) convertV1toV2(rule *model.RoutingConfig) (bool, []*model.ExtendRouterConfig, error) {
	svc := rc.serviceCache.GetServiceByID(rule.ID)
	if svc == nil {
		s, err := rc.storage.GetServiceByID(rule.ID)
		if err != nil {
			return false, nil, err
		}
		if s == nil {
			return true, nil, nil
		}
		svc = s
	}
	if svc.IsAlias() {
		return false, nil, fmt.Errorf("svc: %+v is alias", svc)
	}

	in, out, err := model.ConvertRoutingV1ToExtendV2(svc.Name, svc.Namespace, rule)
	if err != nil {
		return false, nil, err
	}

	ret := make([]*model.ExtendRouterConfig, 0, len(in)+len(out))
	ret = append(ret, in...)
	ret = append(ret, out...)

	return true, ret, nil
}

// convertV2toV1 The routing rules of the V2 version are converted to V1 version to return to the client,
// which is used to compatible with SDK issuance configuration.
func (rc *routingConfigCache) convertV2toV1(entries map[routingLevel][]*model.ExtendRouterConfig,
	service, namespace string) ([]*apitraffic.Route, []*apitraffic.Route, []string) {
	level1 := entries[level1RoutingV2]
	sort.Slice(level1, func(i, j int) bool {
		return model.CompareRoutingV2(level1[i], level1[j])
	})

	level2 := entries[level2RoutingV2]
	sort.Slice(level2, func(i, j int) bool {
		return model.CompareRoutingV2(level2[i], level2[j])
	})

	level3 := entries[level3RoutingV2]
	sort.Slice(level3, func(i, j int) bool {
		return model.CompareRoutingV2(level3[i], level3[j])
	})

	level1inRoutes, level1outRoutes, level1Revisions := model.BuildV1RoutesFromV2(service, namespace, level1)
	level2inRoutes, level2outRoutes, level2Revisions := model.BuildV1RoutesFromV2(service, namespace, level2)
	level3inRoutes, level3outRoutes, level3Revisions := model.BuildV1RoutesFromV2(service, namespace, level3)

	revisions := make([]string, 0, len(level1Revisions)+len(level2Revisions)+len(level3Revisions))
	revisions = append(revisions, level1Revisions...)
	revisions = append(revisions, level2Revisions...)
	revisions = append(revisions, level3Revisions...)

	inRoutes := make([]*apitraffic.Route, 0, len(level1inRoutes)+len(level2inRoutes)+len(level3inRoutes))
	inRoutes = append(inRoutes, level1inRoutes...)
	inRoutes = append(inRoutes, level2inRoutes...)
	inRoutes = append(inRoutes, level3inRoutes...)

	outRoutes := make([]*apitraffic.Route, 0, len(level1outRoutes)+len(level2outRoutes)+len(level3outRoutes))
	outRoutes = append(outRoutes, level1outRoutes...)
	outRoutes = append(outRoutes, level2outRoutes...)
	outRoutes = append(outRoutes, level3outRoutes...)

	return inRoutes, outRoutes, revisions
}
