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

package config_auth

import (
	"context"
	"fmt"

	apiconfig "github.com/polarismesh/specification/source/go/api/v1/config_manage"
	apisecurity "github.com/polarismesh/specification/source/go/api/v1/security"

	api "github.com/polarismesh/polaris/common/api/v1"
	"github.com/polarismesh/polaris/common/model"
	"github.com/polarismesh/polaris/common/utils"
)

// CreateConfigFileGroup 创建配置文件组
func (s *ServerAuthability) CreateConfigFileGroup(ctx context.Context,
	configFileGroup *apiconfig.ConfigFileGroup) *apiconfig.ConfigResponse {
	authCtx := s.collectConfigGroupAuthContext(ctx, []*apiconfig.ConfigFileGroup{configFileGroup},
		model.Create, "CreateConfigFileGroup")

	// 验证 token 信息
	if _, err := s.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewConfigResponseWithInfo(model.ConvertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return s.nextServer.CreateConfigFileGroup(ctx, configFileGroup)
}

// QueryConfigFileGroups 查询配置文件组
func (s *ServerAuthability) QueryConfigFileGroups(ctx context.Context,
	filter map[string]string) *apiconfig.ConfigBatchQueryResponse {

	authCtx := s.collectConfigGroupAuthContext(ctx, nil, model.Read, "QueryConfigFileGroups")

	if _, err := s.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewConfigBatchQueryResponse(model.ConvertToErrCode(err))
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	resp := s.nextServer.QueryConfigFileGroups(ctx, filter)
	if len(resp.ConfigFileGroups) != 0 {
		for index := range resp.ConfigFileGroups {
			group := resp.ConfigFileGroups[index]
			editable := s.policyMgr.GetAuthChecker().AllowResourceOperate(authCtx, &model.ResourceOpInfo{
				ResourceType: apisecurity.ResourceType_ConfigGroups,
				Namespace:    group.GetNamespace().GetValue(),
				ResourceName: group.GetName().GetValue(),
				ResourceID:   fmt.Sprintf("%d", group.GetId().GetValue()),
				Operation:    authCtx.GetOperation(),
			})
			// 如果包含特殊标签，也不允许修改
			if _, ok := group.GetMetadata()[model.MetaKey3RdPlatform]; ok {
				editable = false
			}
			group.Editable = utils.NewBoolValue(editable)
		}
	}
	return resp
}

// DeleteConfigFileGroup 删除配置文件组
func (s *ServerAuthability) DeleteConfigFileGroup(
	ctx context.Context, namespace, name string) *apiconfig.ConfigResponse {
	authCtx := s.collectConfigGroupAuthContext(ctx, []*apiconfig.ConfigFileGroup{{Name: utils.NewStringValue(name),
		Namespace: utils.NewStringValue(namespace)}}, model.Delete, "DeleteConfigFileGroup")

	if _, err := s.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewConfigResponseWithInfo(model.ConvertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return s.nextServer.DeleteConfigFileGroup(ctx, namespace, name)
}

// UpdateConfigFileGroup 更新配置文件组
func (s *ServerAuthability) UpdateConfigFileGroup(ctx context.Context,
	configFileGroup *apiconfig.ConfigFileGroup) *apiconfig.ConfigResponse {
	authCtx := s.collectConfigGroupAuthContext(ctx, []*apiconfig.ConfigFileGroup{configFileGroup},
		model.Modify, "UpdateConfigFileGroup")

	if _, err := s.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewConfigResponseWithInfo(model.ConvertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)
	return s.nextServer.UpdateConfigFileGroup(ctx, configFileGroup)
}
