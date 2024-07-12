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

package service_auth

import (
	"context"

	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"

	api "github.com/polarismesh/polaris/common/api/v1"
	authcommon "github.com/polarismesh/polaris/common/model/auth"
	"github.com/polarismesh/polaris/common/utils"
)

// CreateInstances create instances
func (svr *ServerAuthAbility) CreateInstances(ctx context.Context,
	reqs []*apiservice.Instance) *apiservice.BatchWriteResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, reqs, authcommon.Create, "CreateInstances")

	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		resp := api.NewResponseWithMsg(convertToErrCode(err), err.Error())
		batchResp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
		api.Collect(batchResp, resp)
		return batchResp
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.CreateInstances(ctx, reqs)
}

// DeleteInstances delete instances
func (svr *ServerAuthAbility) DeleteInstances(ctx context.Context,
	reqs []*apiservice.Instance) *apiservice.BatchWriteResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, reqs, authcommon.Delete, "DeleteInstances")

	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		resp := api.NewResponseWithMsg(convertToErrCode(err), err.Error())
		batchResp := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
		api.Collect(batchResp, resp)
		return batchResp
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.DeleteInstances(ctx, reqs)
}

// DeleteInstancesByHost 目前只允许 super account 进行数据删除
func (svr *ServerAuthAbility) DeleteInstancesByHost(ctx context.Context,
	reqs []*apiservice.Instance) *apiservice.BatchWriteResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, reqs, authcommon.Delete, "DeleteInstancesByHost")

	if _, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx); err != nil {
		return api.NewBatchWriteResponse(convertToErrCode(err))
	}
	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)
	if authcommon.ParseUserRole(ctx) == authcommon.SubAccountUserRole {
		ret := api.NewBatchWriteResponse(apimodel.Code_ExecuteSuccess)
		api.Collect(ret, api.NewResponse(apimodel.Code_NotAllowedAccess))
		return ret
	}

	return svr.nextSvr.DeleteInstancesByHost(ctx, reqs)
}

// UpdateInstances update instances
func (svr *ServerAuthAbility) UpdateInstances(ctx context.Context,
	reqs []*apiservice.Instance) *apiservice.BatchWriteResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, reqs, authcommon.Modify, "UpdateInstances")

	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		return api.NewBatchWriteResponseWithMsg(convertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.UpdateInstances(ctx, reqs)
}

// UpdateInstancesIsolate update instances
func (svr *ServerAuthAbility) UpdateInstancesIsolate(ctx context.Context,
	reqs []*apiservice.Instance) *apiservice.BatchWriteResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, reqs, authcommon.Modify, "UpdateInstancesIsolate")

	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		return api.NewBatchWriteResponseWithMsg(convertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.UpdateInstancesIsolate(ctx, reqs)
}

// GetInstances get instances
func (svr *ServerAuthAbility) GetInstances(ctx context.Context,
	query map[string]string) *apiservice.BatchQueryResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, nil, authcommon.Read, "GetInstances")
	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		return api.NewBatchQueryResponseWithMsg(convertToErrCode(err), err.Error())
	}

	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.GetInstances(ctx, query)
}

// GetInstancesCount get instances to count
func (svr *ServerAuthAbility) GetInstancesCount(ctx context.Context) *apiservice.BatchQueryResponse {
	authCtx := svr.collectInstanceAuthContext(ctx, nil, authcommon.Read, "GetInstancesCount")
	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		return api.NewBatchQueryResponseWithMsg(convertToErrCode(err), err.Error())
	}
	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)

	return svr.nextSvr.GetInstancesCount(ctx)
}

func (svr *ServerAuthAbility) GetInstanceLabels(ctx context.Context,
	query map[string]string) *apiservice.Response {

	authCtx := svr.collectInstanceAuthContext(ctx, nil, authcommon.Read, "GetInstanceLabels")
	_, err := svr.policyMgr.GetAuthChecker().CheckConsolePermission(authCtx)
	if err != nil {
		return api.NewResponseWithMsg(convertToErrCode(err), err.Error())
	}
	ctx = authCtx.GetRequestContext()
	ctx = context.WithValue(ctx, utils.ContextAuthContextKey, authCtx)
	return svr.nextSvr.GetInstanceLabels(ctx, query)
}
