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

package auth

import (
	"fmt"
	"math"
	"time"

	apisecurity "github.com/polarismesh/specification/source/go/api/v1/security"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	types "github.com/polarismesh/polaris/cache/api"
	authcommon "github.com/polarismesh/polaris/common/model/auth"
	"github.com/polarismesh/polaris/common/utils"
	"github.com/polarismesh/polaris/store"
)

const (
	removePrincipalChSize = 8
)

// strategyCache
type strategyCache struct {
	*types.BaseCache

	storage          store.Store
	strategys        *utils.SyncMap[string, *authcommon.StrategyDetailCache]
	uid2Strategy     *utils.SyncMap[string, *utils.SyncSet[string]]
	groupid2Strategy *utils.SyncMap[string, *utils.SyncSet[string]]

	namespace2Strategy   *utils.SyncMap[string, *utils.SyncSet[string]]
	service2Strategy     *utils.SyncMap[string, *utils.SyncSet[string]]
	configGroup2Strategy *utils.SyncMap[string, *utils.SyncSet[string]]

	lastMtime    int64
	userCache    *userCache
	singleFlight *singleflight.Group
}

// NewStrategyCache
func NewStrategyCache(storage store.Store, cacheMgr types.CacheManager) types.StrategyCache {
	return &strategyCache{
		BaseCache: types.NewBaseCache(storage, cacheMgr),
		storage:   storage,
	}
}

func (sc *strategyCache) Initialize(c map[string]interface{}) error {
	sc.userCache = sc.BaseCache.CacheMgr.GetCacher(types.CacheUser).(*userCache)
	sc.strategys = utils.NewSyncMap[string, *authcommon.StrategyDetailCache]()
	sc.uid2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.groupid2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.namespace2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.service2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.configGroup2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.singleFlight = new(singleflight.Group)
	sc.lastMtime = 0
	return nil
}

func (sc *strategyCache) Update() error {
	// 多个线程竞争，只有一个线程进行更新
	_, err, _ := sc.singleFlight.Do(sc.Name(), func() (interface{}, error) {
		return nil, sc.DoCacheUpdate(sc.Name(), sc.realUpdate)
	})
	return err
}

func (sc *strategyCache) ForceSync() error {
	return sc.Update()
}

func (sc *strategyCache) realUpdate() (map[string]time.Time, int64, error) {
	// 获取几秒前的全部数据
	var (
		start           = time.Now()
		lastTime        = sc.LastFetchTime()
		strategies, err = sc.storage.GetStrategyDetailsForCache(lastTime, sc.IsFirstUpdate())
	)
	if err != nil {
		log.Errorf("[Cache][AuthStrategy] refresh auth strategy cache err: %s", err.Error())
		return nil, -1, err
	}

	lastMtimes, add, update, del := sc.setStrategys(strategies)
	log.Info("[Cache][AuthStrategy] get more auth strategy",
		zap.Int("add", add), zap.Int("update", update), zap.Int("delete", del),
		zap.Time("last", lastTime), zap.Duration("used", time.Since(start)))
	return lastMtimes, int64(len(strategies)), nil
}

// setStrategys 处理策略的数据更新情况
// step 1. 先处理resource以及principal的数据更新情况（主要是为了能够获取到新老数据进行对比计算）
// step 2. 处理真正的 strategy 的缓存更新
func (sc *strategyCache) setStrategys(strategies []*authcommon.StrategyDetail) (map[string]time.Time, int, int, int) {
	var add, remove, update int

	sc.handlerResourceStrategy(strategies)
	sc.handlerPrincipalStrategy(strategies)

	lastMtime := sc.LastMtime(sc.Name()).Unix()

	for index := range strategies {
		rule := strategies[index]
		if !rule.Valid {
			sc.strategys.Delete(rule.ID)
			remove++
		} else {
			_, ok := sc.strategys.Load(rule.ID)
			if !ok {
				add++
			} else {
				update++
			}
			sc.strategys.Store(rule.ID, buildEnchanceStrategyDetail(rule))
		}

		lastMtime = int64(math.Max(float64(lastMtime), float64(rule.ModifyTime.Unix())))
	}

	return map[string]time.Time{sc.Name(): time.Unix(lastMtime, 0)}, add, update, remove
}

func buildEnchanceStrategyDetail(strategy *authcommon.StrategyDetail) *authcommon.StrategyDetailCache {
	users := make(map[string]authcommon.Principal, 0)
	groups := make(map[string]authcommon.Principal, 0)

	for index := range strategy.Principals {
		principal := strategy.Principals[index]
		if principal.PrincipalRole == authcommon.PrincipalUser {
			users[principal.PrincipalID] = principal
		} else {
			groups[principal.PrincipalID] = principal
		}
	}

	return &authcommon.StrategyDetailCache{
		StrategyDetail: strategy,
		UserPrincipal:  users,
		GroupPrincipal: groups,
	}
}

func (sc *strategyCache) writeSet(linkContainers *utils.SyncMap[string, *utils.SyncSet[string]], key, val string, isDel bool) {
	if isDel {
		values, ok := linkContainers.Load(key)
		if ok {
			values.Remove(val)
		}
	} else {
		if _, ok := linkContainers.Load(key); !ok {
			linkContainers.Store(key, utils.NewSyncSet[string]())
		}
		values, _ := linkContainers.Load(key)
		values.Add(val)
	}
}

// handlerResourceStrategy 处理资源视角下策略的缓存
// 根据新老策略的资源列表比对，计算出哪些资源不在和该策略存在关联关系，哪些资源新增了相关的策略
func (sc *strategyCache) handlerResourceStrategy(strategies []*authcommon.StrategyDetail) {
	operateLink := func(resType int32, resId, strategyId string, remove bool) {
		switch resType {
		case int32(apisecurity.ResourceType_Namespaces):
			sc.writeSet(sc.namespace2Strategy, resId, strategyId, remove)
		case int32(apisecurity.ResourceType_Services):
			sc.writeSet(sc.service2Strategy, resId, strategyId, remove)
		case int32(apisecurity.ResourceType_ConfigGroups):
			sc.writeSet(sc.configGroup2Strategy, resId, strategyId, remove)
		}
	}

	for sIndex := range strategies {
		rule := strategies[sIndex]
		addRes := rule.Resources

		if oldRule, exist := sc.strategys.Load(rule.ID); exist {
			delRes := make([]authcommon.StrategyResource, 0, 8)
			// 计算前后对比， resource 的变化
			newRes := make(map[string]struct{}, len(addRes))
			for i := range addRes {
				newRes[fmt.Sprintf("%d_%s", addRes[i].ResType, addRes[i].ResID)] = struct{}{}
			}

			// 筛选出从策略中被踢出的 resource 列表
			for i := range oldRule.Resources {
				item := oldRule.Resources[i]
				if _, ok := newRes[fmt.Sprintf("%d_%s", item.ResType, item.ResID)]; !ok {
					delRes = append(delRes, item)
				}
			}

			// 针对被剔除的 resource 列表，清理掉所关联的鉴权策略信息
			for rIndex := range delRes {
				resource := delRes[rIndex]
				operateLink(resource.ResType, resource.ResID, rule.ID, true)
			}
		}

		for rIndex := range addRes {
			resource := addRes[rIndex]
			if rule.Valid {
				operateLink(resource.ResType, resource.ResID, rule.ID, false)
			} else {
				operateLink(resource.ResType, resource.ResID, rule.ID, true)
			}
		}
	}
}

// handlerPrincipalStrategy
func (sc *strategyCache) handlerPrincipalStrategy(strategies []*authcommon.StrategyDetail) {
	for index := range strategies {
		rule := strategies[index]
		// 计算 uid -> auth rule
		principals := rule.Principals

		if oldRule, exist := sc.strategys.Load(rule.ID); exist {
			delMembers := make([]authcommon.Principal, 0, 8)
			// 计算前后对比， principal 的变化
			newRes := make(map[string]struct{}, len(principals))
			for i := range principals {
				newRes[fmt.Sprintf("%d_%s", principals[i].PrincipalRole, principals[i].PrincipalID)] = struct{}{}
			}

			// 筛选出从策略中被踢出的 principal 列表
			for i := range oldRule.Principals {
				item := oldRule.Principals[i]
				if _, ok := newRes[fmt.Sprintf("%d_%s", item.PrincipalRole, item.PrincipalID)]; !ok {
					delMembers = append(delMembers, item)
				}
			}

			// 针对被剔除的 principal 列表，清理掉所关联的鉴权策略信息
			for rIndex := range delMembers {
				principal := delMembers[rIndex]
				sc.removePrincipalLink(principal, rule)
			}
		}
		if rule.Valid {
			for pos := range principals {
				principal := principals[pos]
				sc.addPrincipalLink(principal, rule)
			}
		} else {
			for pos := range principals {
				principal := principals[pos]
				sc.removePrincipalLink(principal, rule)
			}
		}
	}
}

func (sc *strategyCache) removePrincipalLink(principal authcommon.Principal, rule *authcommon.StrategyDetail) {
	linkContainers := sc.uid2Strategy
	if principal.PrincipalRole != authcommon.PrincipalUser {
		linkContainers = sc.groupid2Strategy
	}
	sc.writeSet(linkContainers, principal.PrincipalID, rule.ID, true)
}

func (sc *strategyCache) addPrincipalLink(principal authcommon.Principal, rule *authcommon.StrategyDetail) {
	linkContainers := sc.uid2Strategy
	if principal.PrincipalRole != authcommon.PrincipalUser {
		linkContainers = sc.groupid2Strategy
	}
	sc.writeSet(linkContainers, principal.PrincipalID, rule.ID, false)
}

func (sc *strategyCache) Clear() error {
	sc.BaseCache.Clear()
	sc.strategys = utils.NewSyncMap[string, *authcommon.StrategyDetailCache]()
	sc.uid2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.groupid2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.namespace2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.service2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.configGroup2Strategy = utils.NewSyncMap[string, *utils.SyncSet[string]]()
	sc.lastMtime = 0
	return nil
}

func (sc *strategyCache) Name() string {
	return types.StrategyRuleName
}

// 对于 check 逻辑，如果是计算 * 策略，则必须要求 * 资源下必须有策略
// 如果是具体的资源ID，则该资源下不必有策略，如果没有策略就认为这个资源是可以被任何人编辑的
func (sc *strategyCache) checkResourceEditable(strategIds *utils.SyncSet[string], principal authcommon.Principal, mustCheck bool) bool {
	// 是否可以编辑
	editable := false
	// 是否真的包含策略
	isCheck := strategIds.Len() != 0

	// 如果根本没有遍历过，则表示该资源下没有对应的策略列表，直接返回可编辑状态即可
	if !isCheck && !mustCheck {
		return true
	}

	strategIds.Range(func(strategyId string) {
		isCheck = true
		if rule, ok := sc.strategys.Load(strategyId); ok {
			if principal.PrincipalRole == authcommon.PrincipalUser {
				_, exist := rule.UserPrincipal[principal.PrincipalID]
				editable = editable || exist
			} else {
				_, exist := rule.GroupPrincipal[principal.PrincipalID]
				editable = editable || exist
			}
		}
	})

	return editable
}

// IsResourceEditable 判断当前资源是否可以操作
// 这里需要考虑两种情况，一种是 “ * ” 策略，另一种是明确指出了具体的资源ID的策略
func (sc *strategyCache) IsResourceEditable(
	principal authcommon.Principal, resType apisecurity.ResourceType, resId string) bool {
	var (
		valAll, val *utils.SyncSet[string]
		ok          bool
	)
	switch resType {
	case apisecurity.ResourceType_Namespaces:
		val, ok = sc.namespace2Strategy.Load(resId)
		valAll, _ = sc.namespace2Strategy.Load("*")
	case apisecurity.ResourceType_Services:
		val, ok = sc.service2Strategy.Load(resId)
		valAll, _ = sc.service2Strategy.Load("*")
	case apisecurity.ResourceType_ConfigGroups:
		val, ok = sc.configGroup2Strategy.Load(resId)
		valAll, _ = sc.configGroup2Strategy.Load("*")
	}

	// 代表该资源没有关联到任何策略，任何人都可以编辑
	if !ok {
		return true
	}

	principals := make([]authcommon.Principal, 0, 4)
	principals = append(principals, principal)
	if principal.PrincipalRole == authcommon.PrincipalUser {
		groupids := sc.userCache.GetUserLinkGroupIds(principal.PrincipalID)
		for i := range groupids {
			principals = append(principals, authcommon.Principal{
				PrincipalID:   groupids[i],
				PrincipalRole: authcommon.PrincipalGroup,
			})
		}
	}

	for i := range principals {
		item := principals[i]
		if valAll != nil && sc.checkResourceEditable(valAll, item, true) {
			return true
		}

		if sc.checkResourceEditable(val, item, false) {
			return true
		}
	}

	return false
}

func (sc *strategyCache) GetStrategyDetailsByUID(uid string) []*authcommon.StrategyDetail {
	return sc.getStrategyDetails(uid, "")
}

func (sc *strategyCache) GetStrategyDetailsByGroupID(groupid string) []*authcommon.StrategyDetail {
	return sc.getStrategyDetails("", groupid)
}

func (sc *strategyCache) getStrategyDetails(uid string, gid string) []*authcommon.StrategyDetail {
	var (
		strategyIds []string
	)
	if uid != "" {
		sets, ok := sc.uid2Strategy.Load(uid)
		if !ok {
			return nil
		}
		strategyIds = sets.ToSlice()
	} else if gid != "" {
		sets, ok := sc.groupid2Strategy.Load(gid)
		if !ok {
			return nil
		}
		strategyIds = sets.ToSlice()
	}

	result := make([]*authcommon.StrategyDetail, 0, 16)
	if len(strategyIds) > 0 {
		for i := range strategyIds {
			strategy, ok := sc.strategys.Load(strategyIds[i])
			if ok {
				result = append(result, strategy.StrategyDetail)
			}
		}
	}
	return result
}

// IsResourceLinkStrategy 校验
func (sc *strategyCache) IsResourceLinkStrategy(resType apisecurity.ResourceType, resId string) bool {
	hasLinkRule := func(sets *utils.SyncSet[string]) bool {
		return sets.Len() != 0
	}

	switch resType {
	case apisecurity.ResourceType_Namespaces:
		val, ok := sc.namespace2Strategy.Load(resId)
		return ok && hasLinkRule(val)
	case apisecurity.ResourceType_Services:
		val, ok := sc.service2Strategy.Load(resId)
		return ok && hasLinkRule(val)
	case apisecurity.ResourceType_ConfigGroups:
		val, ok := sc.configGroup2Strategy.Load(resId)
		return ok && hasLinkRule(val)
	default:
		return true
	}
}
