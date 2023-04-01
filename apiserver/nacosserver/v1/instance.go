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

package v1

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/polarismesh/polaris/apiserver/nacosserver/core"
	"github.com/polarismesh/polaris/apiserver/nacosserver/model"
	commonmodel "github.com/polarismesh/polaris/common/model"
	"github.com/polarismesh/polaris/common/utils"
	"github.com/polarismesh/polaris/service"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
)

type (
	keyHealthyOnly struct{}
	keyService     struct{}
)

func (n *NacosV1Server) handleRegister(ctx context.Context, namespace, serviceName string, ins *model.Instance) error {
	specIns := model.PrepareSpecInstance(namespace, serviceName, ins)
	ctx = context.WithValue(ctx, service.KeyModifyService{}, func(svc *apiservice.Service) {
		if len(svc.Metadata) == 0 {
			svc.Metadata = map[string]string{}
		}
		svc.Metadata[model.InternalNacosServiceType] = strconv.FormatBool(true)
	})

	resp := n.discoverSvr.RegisterInstance(ctx, specIns)
	if apimodel.Code(resp.GetCode().GetValue()) != apimodel.Code_ExecuteSuccess {
		return &model.NacosError{
			ErrCode: int32(model.ExceptionCode_ServerError),
			ErrMsg:  resp.GetInfo().GetValue(),
		}
	}
	return nil
}

func (n *NacosV1Server) handleDeregister(ctx context.Context, namespace, service string, ins *model.Instance) error {
	specIns := model.PrepareSpecInstance(namespace, service, ins)

	resp := n.discoverSvr.DeregisterInstance(ctx, specIns)
	if apimodel.Code(resp.GetCode().GetValue()) != apimodel.Code_ExecuteSuccess {
		return &model.NacosError{
			ErrCode: int32(model.ExceptionCode_ServerError),
			ErrMsg:  resp.GetInfo().GetValue(),
		}
	}
	return nil
}

// handleBeat com.alibaba.nacos.naming.core.InstanceOperatorClientImpl#handleBeat
func (n *NacosV1Server) handleBeat(ctx context.Context, namespace, service string,
	clientBeat *model.ClientBeat) (map[string]interface{}, error) {
	resp := n.healthSvr.Report(ctx, &apiservice.Instance{
		Service:   utils.NewStringValue(model.ReplaceNacosService(service)),
		Namespace: utils.NewStringValue(namespace),
		Host:      utils.NewStringValue(clientBeat.Ip),
		Port:      utils.NewUInt32Value(uint32(clientBeat.Port)),
	})
	rspCode := apimodel.Code(resp.GetCode().GetValue())

	if rspCode == apimodel.Code_ExecuteSuccess {
		return map[string]interface{}{
			"code":               10200,
			"clientBeatInterval": model.ClientBeatIntervalMill,
			"lightBeatEnabled":   true,
		}, nil
	}

	if rspCode == apimodel.Code_NotFoundResource {
		// TODO 老版本 nacos server 支持通过心跳信息完成服务注册，这里待补充该流程
	}

	return nil, &model.NacosError{
		ErrCode: int32(model.ExceptionCode_ServerError),
		ErrMsg:  resp.GetInfo().GetValue(),
	}

}

// handleQueryInstances com.alibaba.nacos.naming.controllers.InstanceController#list
func (n *NacosV1Server) handleQueryInstances(ctx context.Context, params map[string]string) (interface{}, error) {
	namespace := params[model.ParamNamespaceID]
	service := params[model.ParamServiceName]
	clusters := params["clusters"]
	clientIP, _ := params["clientIP"]
	udpPort, _ := strconv.ParseInt(params["udpPort"], 10, 32)
	healthyOnly, _ := strconv.ParseBool(params["healthyOnly"])

	if n.pushCenter != nil && udpPort > 0 {
		// TODO 加入到 pushCenter 的任务中
		n.pushCenter.AddSubscriber(core.Subscriber{
			Agent:       "",
			App:         "",
			AddrStr:     clientIP,
			Ip:          clientIP,
			Port:        int(udpPort),
			NamespaceId: namespace,
			ServiceName: service,
			Cluster:     clusters,
			Type:        core.UDPCPush,
		})
	}

	ctx = context.WithValue(ctx, keyHealthyOnly{}, healthyOnly)
	// 默认只下发 enable 的实例
	result := n.store.ListInstances(ctx, commonmodel.ServiceKey{
		Namespace: namespace,
		Name:      service,
	}, strings.Split(clusters, ","), selectInstancesWithHealthyProtection)

	// adapt for nacos v1.x SDK
	result.Name = fmt.Sprintf("%s%s%s", result.Name, model.DefaultNacosGroupConnectStr, result.GroupName)

	return result, nil
}

func selectInstancesWithHealthyProtection(ctx context.Context, result *model.ServiceInfo,
	instances []*model.Instance, healthCount int32) *model.ServiceInfo {
	healthyOnly, _ := ctx.Value(keyHealthyOnly{}).(bool)

	checkProtectThreshold := false
	protectThreshold := float64(0)
	svc, _ := ctx.Value(keyService{}).(*commonmodel.Service)
	if svc != nil {
		val, ok := svc.Meta[model.InternalNacosServiceProtectThreshold]
		checkProtectThreshold = ok
		protectThreshold, _ = strconv.ParseFloat(val, 64)
	}

	if !checkProtectThreshold || float64(healthCount)/float64(len(instances)) <= protectThreshold {
		ret := instances
		if healthyOnly {
			healthyIns := make([]*model.Instance, 0, len(instances))
			for i := range instances {
				if instances[i].Healthy {
					healthyIns = append(healthyIns, instances[i])
				}
			}
			ret = healthyIns
		}
		result.Hosts = ret
		return result
	}

	ret := make([]*model.Instance, 0, len(instances))

	for i := range instances {
		if !instances[i].Healthy {
			copyIns := instances[i].DeepClone()
			copyIns.Healthy = true
			ret = append(ret, copyIns)
		} else {
			ret = append(ret, instances[i])
		}
	}

	result.ReachProtectionThreshold = true
	result.Hosts = ret
	return result
}
