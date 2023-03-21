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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/wrappers"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
	apitraffic "github.com/polarismesh/specification/source/go/api/v1/traffic_manage"
	"go.uber.org/zap"

	api "github.com/polarismesh/polaris/common/api/v1"
	"github.com/polarismesh/polaris/common/metrics"
	"github.com/polarismesh/polaris/common/model"
	"github.com/polarismesh/polaris/common/utils"
)

// RegisterInstance create one instance
func (s *Server) RegisterInstance(ctx context.Context, req *apiservice.Instance) *apiservice.Response {
	ctx = context.WithValue(ctx, utils.ContextIsFromClient, true)
	return s.CreateInstance(ctx, req)
}

// DeregisterInstance delete one instance
func (s *Server) DeregisterInstance(ctx context.Context, req *apiservice.Instance) *apiservice.Response {
	ctx = context.WithValue(ctx, utils.ContextIsFromClient, true)
	return s.DeleteInstance(ctx, req)
}

// ReportClient 客户端上报信息
func (s *Server) ReportClient(ctx context.Context, req *apiservice.Client) *apiservice.Response {
	if s.caches == nil {
		return api.NewResponse(apimodel.Code_ClientAPINotOpen)
	}

	// 客户端信息不写入到DB中
	host := req.GetHost().GetValue()
	// 从CMDB查询地理位置信息
	if s.cmdb != nil {
		location, err := s.cmdb.GetLocation(host)
		if err != nil {
			log.Errora(utils.ZapRequestIDByCtx(ctx), zap.Error(err))
		}
		if location != nil {
			req.Location = location.Proto
		}
	}

	// save the client with unique id into store
	if len(req.GetId().GetValue()) > 0 {
		return s.checkAndStoreClient(ctx, req)
	}
	out := &apiservice.Client{
		Host:     req.GetHost(),
		Location: req.Location,
	}
	return api.NewClientResponse(apimodel.Code_ExecuteSuccess, out)
}

// GetPrometheusTargets Used for client acquisition service information
func (s *Server) GetPrometheusTargets(ctx context.Context,
	query map[string]string) *model.PrometheusDiscoveryResponse {
	if s.caches == nil {
		return &model.PrometheusDiscoveryResponse{
			Code:     api.NotFoundInstance,
			Response: make([]model.PrometheusTarget, 0),
		}
	}

	targets := make([]model.PrometheusTarget, 0, 8)
	expectSchema := map[string]struct{}{
		"http":  {},
		"https": {},
	}

	s.Cache().Client().IteratorClients(func(key string, value *model.Client) bool {
		for i := range value.Proto().Stat {
			stat := value.Proto().Stat[i]
			if stat.Target.GetValue() != model.StatReportPrometheus {
				continue
			}
			_, ok := expectSchema[strings.ToLower(stat.Protocol.GetValue())]
			if !ok {
				continue
			}

			target := model.PrometheusTarget{
				Targets: []string{fmt.Sprintf("%s:%d", value.Proto().Host.GetValue(), stat.Port.GetValue())},
				Labels: map[string]string{
					"__metrics_path__":         stat.Path.GetValue(),
					"__scheme__":               stat.Protocol.GetValue(),
					"__meta_polaris_client_id": value.Proto().Id.GetValue(),
				},
			}
			targets = append(targets, target)
		}

		return true
	})

	// 加入北极星集群自身
	checkers := s.healthServer.ListCheckerServer()
	for i := range checkers {
		checker := checkers[i]
		target := model.PrometheusTarget{
			Targets: []string{fmt.Sprintf("%s:%d", checker.Host(), metrics.GetMetricsPort())},
			Labels: map[string]string{
				"__metrics_path__":         "/metrics",
				"__scheme__":               "http",
				"__meta_polaris_client_id": checker.ID(),
			},
		}
		targets = append(targets, target)
	}

	return &model.PrometheusDiscoveryResponse{
		Code:     api.ExecuteSuccess,
		Response: targets,
	}
}

// GetServiceWithCache 查询服务列表
func (s *Server) GetServiceWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverServiceResponse(apimodel.Code_ClientAPINotOpen, req)
	}
	if req == nil {
		return api.NewDiscoverServiceResponse(apimodel.Code_EmptyRequest, req)
	}

	resp := api.NewDiscoverServiceResponse(apimodel.Code_ExecuteSuccess, req)
<<<<<<< HEAD
	var (
		revision string
		svcs     []*model.Service
	)

	if req.GetNamespace().GetValue() != "" {
		revision, svcs = s.Cache().Service().ListServices(req.GetNamespace().GetValue())
	} else {
		revision, svcs = s.Cache().Service().ListAllServices()
	}
	if revision == "" {
		return resp
	}

	log.Info("[Service][Discover] list servies", zap.Int("size", len(svcs)), zap.String("revision", revision))
	if revision == req.GetRevision().GetValue() {
		return api.NewDiscoverServiceResponse(apimodel.Code_DataNoChange, req)
=======
	resp.Services = []*apiservice.Service{}
	serviceIterProc := func(key string, value *model.Service) (bool, error) {
		if checkServiceMetadata(req.GetMetadata(), value, req.Business.GetValue(), req.Namespace.GetValue()) {
			service := &apiservice.Service{
				Name:      utils.NewStringValue(value.Name),
				Namespace: utils.NewStringValue(value.Namespace),
			}
			resp.Services = append(resp.Services, service)
		}
		return true, nil
	}

	if err := s.caches.Service().IteratorServices(serviceIterProc); err != nil {
		log.Error(err.Error(), utils.ZapRequestIDByCtx(ctx))
		return api.NewDiscoverServiceResponse(apimodel.Code_ExecuteException, req)
	}

	return resp
}

// checkServiceMetadata 判断请求元数据是否属于服务的元数据
func checkServiceMetadata(requestMeta map[string]string, service *model.Service, business, namespace string) bool {
	if len(business) > 0 && business != service.Business {
		return false
>>>>>>> 4fb231a7... fix:查询治理规则返回服务原始名称
	}

	ret := make([]*apiservice.Service, 0, len(svcs))
	for i := range svcs {
		ret = append(ret, &apiservice.Service{
			Namespace: utils.NewStringValue(svcs[i].Namespace),
			Name:      utils.NewStringValue(svcs[i].Name),
			Metadata:  svcs[i].Meta,
		})
	}

	resp.Services = ret
	resp.Service = &apiservice.Service{
		Namespace: utils.NewStringValue(req.GetNamespace().GetValue()),
		Name:      utils.NewStringValue(req.GetName().GetValue()),
		Revision:  utils.NewStringValue(revision),
	}

	return resp
}

// ServiceInstancesCache 根据服务名查询服务实例列表
func (s *Server) ServiceInstancesCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if req == nil {
		return api.NewDiscoverInstanceResponse(apimodel.Code_EmptyRequest, req)
	}
	if s.caches == nil {
		return api.NewDiscoverInstanceResponse(apimodel.Code_ClientAPINotOpen, req)
	}

	serviceName := req.GetName().GetValue()
	namespaceName := req.GetNamespace().GetValue()

	if serviceName == "" {
		return api.NewDiscoverInstanceResponse(apimodel.Code_InvalidServiceName, req)
	}

	// 消费服务为了兼容，可以不带namespace，server端使用默认的namespace
	if namespaceName == "" {
		namespaceName = DefaultNamespace
	}

	// 数据源都来自Cache，这里拿到的service，已经是源服务
	service := s.getServiceCache(serviceName, namespaceName)
	if service == nil {
		log.Errorf("[Server][Service][Instance] not found name(%s) namespace(%s) service", serviceName, namespaceName)
		return api.NewDiscoverInstanceResponse(apimodel.Code_NotFoundResource, req)
	}
	s.RecordDiscoverStatis(service.Name, service.Namespace)
	// 获取revision，如果revision一致，则不返回内容，直接返回一个状态码
	revision := s.caches.GetServiceInstanceRevision(service.ID)
	if revision == "" {
		// 不能直接获取，则需要重新计算，大部分情况都可以直接获取的
		// 获取instance数据，service已经是源服务，可以直接查找cache
		instances := s.caches.Instance().GetInstancesByServiceID(service.ID)
		var revisionErr error
		revision, revisionErr = s.GetServiceInstanceRevision(service.ID, instances)
		if revisionErr != nil {
			log.Errorf("[Server][Service][Instance] compute revision service(%s) err: %s",
				service.ID, revisionErr.Error())
			return api.NewDiscoverInstanceResponse(apimodel.Code_ExecuteException, req)
		}
	}
	if revision == req.GetRevision().GetValue() {
		return api.NewDiscoverInstanceResponse(apimodel.Code_DataNoChange, req)
	}

	// revision不一致，重新获取数据
	// 填充service数据
	resp := api.NewDiscoverInstanceResponse(apimodel.Code_ExecuteSuccess, service2Api(service))
	// 填充新的revision TODO
	resp.Service.Revision.Value = revision
	resp.Service.Namespace = req.GetNamespace()
	resp.Service.Name = req.GetName() // 别名场景，response需要保持和request的服务名一致
	// 塞入源服务信息数据
	resp.AliasFor = &apiservice.Service{
		Namespace: utils.NewStringValue(service.Namespace),
		Name:      utils.NewStringValue(service.Name),
	}
	// 填充instance数据
	resp.Instances = make([]*apiservice.Instance, 0) // TODO
	_ = s.caches.Instance().
		IteratorInstancesWithService(service.ID, // service已经是源服务
			func(key string, value *model.Instance) (b bool, e error) {
				// 注意：这里的value是cache的，不修改cache的数据，通过getInstance，浅拷贝一份数据
				resp.Instances = append(resp.Instances, s.getInstance(req, value.Proto))
				return true, nil
			})

	return resp
}

// GetRoutingConfigWithCache 获取缓存中的路由配置信息
func (s *Server) GetRoutingConfigWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_ClientAPINotOpen, req)
	}
	if req == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_EmptyRequest, req)
	}

	if req.GetName().GetValue() == "" {
		return api.NewDiscoverRoutingResponse(apimodel.Code_InvalidServiceName, req)
	}
	if req.GetNamespace().GetValue() == "" {
		return api.NewDiscoverRoutingResponse(apimodel.Code_InvalidNamespaceName, req)
	}

	resp := api.NewDiscoverRoutingResponse(apimodel.Code_ExecuteSuccess, nil)
	resp.Service = &apiservice.Service{
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	}

	// 先从缓存获取ServiceID，这里返回的是源服务
	svc := s.getServiceCache(req.GetName().GetValue(), req.GetNamespace().GetValue())
	if svc == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_NotFoundService, req)
	}
	// 塞入源服务信息数据
	resp.AliasFor = &apiservice.Service{
		Namespace: utils.NewStringValue(svc.Namespace),
		Name:      utils.NewStringValue(svc.Name),
	}
	out, err := s.caches.RoutingConfig().GetRoutingConfigV1(svc.ID, svc.Name, svc.Namespace)
	if err != nil {
		log.Error("[Server][Service][Routing] discover routing", utils.ZapRequestIDByCtx(ctx), zap.Error(err))
		return api.NewDiscoverRoutingResponse(apimodel.Code_ExecuteException, req)
	}

	if out == nil {
		return resp
	}

	// 获取路由数据，并对比revision
	if out.GetRevision().GetValue() == req.GetRevision().GetValue() {
		return api.NewDiscoverRoutingResponse(apimodel.Code_DataNoChange, req)
	}

	// 数据不一致，发生了改变
	// 数据格式转换，service只需要返回二元组与routing的revision
	resp.Service.Revision = out.GetRevision()
	resp.Routing = out
	return resp
}

// GetRateLimitWithCache 获取缓存中的限流规则信息
func (s *Server) GetRateLimitWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_ClientAPINotOpen, req)
	}

	if req == nil {
		return api.NewDiscoverRateLimitResponse(apimodel.Code_EmptyRequest, req)
	}
	if req.GetName().GetValue() == "" {
		return api.NewDiscoverRateLimitResponse(apimodel.Code_InvalidServiceName, req)
	}
	if req.GetNamespace().GetValue() == "" {
		return api.NewDiscoverRateLimitResponse(apimodel.Code_InvalidNamespaceName, req)
	}

	// 获取源服务
	service := s.getServiceCache(req.GetName().GetValue(), req.GetNamespace().GetValue())
	if service == nil {
		return api.NewDiscoverRateLimitResponse(apimodel.Code_NotFoundService, req)
	}

	resp := api.NewDiscoverRateLimitResponse(apimodel.Code_ExecuteSuccess, nil)
	// 塞入源服务信息数据
	resp.AliasFor = &apiservice.Service{
		Namespace: utils.NewStringValue(service.Namespace),
		Name:      utils.NewStringValue(service.Name),
	}
	// 服务名和request保持一致
	resp.Service = &apiservice.Service{
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	}

	// 获取最新的revision
	lastRevision := s.caches.RateLimit().GetLastRevision(service.ID)

	// 缓存中无此服务的限流规则
	if lastRevision == "" {
		return resp
	}

	if req.GetRevision().GetValue() == lastRevision {
		return api.NewDiscoverRateLimitResponse(apimodel.Code_DataNoChange, req)
	}

	// 获取限流规则数据
	resp.Service.Revision = utils.NewStringValue(lastRevision) // 更新revision

	resp.RateLimit = &apitraffic.RateLimit{
		Revision: utils.NewStringValue(lastRevision),
		Rules:    []*apitraffic.Rule{},
	}

	rateLimitIterProc := func(key string, value *model.RateLimit) (bool, error) {
		rateLimit, err := rateLimit2Client(req.GetName().GetValue(), req.GetNamespace().GetValue(), value)
		if err != nil {
			return false, err
		}
		if rateLimit == nil {
			return false, errors.New("ratelimit is nil")
		}
		resp.RateLimit.Rules = append(resp.RateLimit.Rules, rateLimit)
		return true, nil
	}

	err := s.caches.RateLimit().GetRateLimit(service.ID, rateLimitIterProc)
	if err != nil {
		log.Error(err.Error(), utils.ZapRequestIDByCtx(ctx))
		return api.NewDiscoverRateLimitResponse(apimodel.Code_ExecuteException, req)
	}

	return resp
}

func (s *Server) GetFaultDetectWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_ClientAPINotOpen, req)
	}
	if req == nil {
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_EmptyRequest, req)
	}
	if req.GetName().GetValue() == "" {
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_InvalidServiceName, req)
	}
	if req.GetNamespace().GetValue() == "" {
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_InvalidNamespaceName, req)
	}
	resp := api.NewDiscoverFaultDetectorResponse(apimodel.Code_ExecuteSuccess, nil)
	// 服务名和request保持一致
	resp.Service = &apiservice.Service{
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	}

	out := s.caches.FaultDetector().GetFaultDetectConfig(req.GetName().GetValue(), req.GetNamespace().GetValue())
	if out == nil || out.Revision == "" {
		return resp
	}

	if req.GetRevision().GetValue() == out.Revision {
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_DataNoChange, req)
	}

	// 数据不一致，发生了改变
	var err error
	resp.Service.Revision = utils.NewStringValue(out.Revision)
	resp.FaultDetector, err = faultDetectRule2ClientAPI(out)
	if err != nil {
		log.Error(err.Error(), utils.ZapRequestID(requestID))
		return api.NewDiscoverFaultDetectorResponse(apimodel.Code_ExecuteException, req)
	}
	return resp
}

// GetCircuitBreakerWithCache 获取缓存中的熔断规则信息
func (s *Server) GetCircuitBreakerWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_ClientAPINotOpen, req)
	}
	if req == nil {
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_EmptyRequest, req)
	}

	if req.GetName().GetValue() == "" {
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_InvalidServiceName, req)
	}
	if req.GetNamespace().GetValue() == "" {
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_InvalidNamespaceName, req)
	}

	resp := api.NewDiscoverCircuitBreakerResponse(apimodel.Code_ExecuteSuccess, nil)
	// 服务名和request保持一致
	resp.Service = &apiservice.Service{
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	}

	out := s.caches.CircuitBreaker().GetCircuitBreakerConfig(req.GetName().GetValue(), req.GetNamespace().GetValue())
	if out == nil || out.Revision == "" {
		return resp
	}

	// 获取熔断规则数据，并对比revision
	if len(req.GetRevision().GetValue()) > 0 && req.GetRevision().GetValue() == out.Revision {
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_DataNoChange, req)
	}

	// 数据不一致，发生了改变
	var err error
	resp.Service.Revision = utils.NewStringValue(out.Revision)
	resp.CircuitBreaker, err = circuitBreaker2ClientAPI(out, req.GetName().GetValue(), req.GetNamespace().GetValue())
	if err != nil {
		log.Error(err.Error(), utils.ZapRequestIDByCtx(ctx))
		return api.NewDiscoverCircuitBreakerResponse(apimodel.Code_ExecuteException, req)
	}
	return resp
}

// 根据ServiceID获取instances
func (s *Server) getInstancesCache(service *model.Service) []*model.Instance {
	id := s.getSourceServiceID(service)
	// TODO refer_filter还要处理一下
	return s.caches.Instance().GetInstancesByServiceID(id)
}

// 获取顶级服务ID
// 没有顶级ID，则返回自身
func (s *Server) getSourceServiceID(service *model.Service) string {
	if service == nil || service.ID == "" {
		return ""
	}
	// 找到parent服务，最多两级，因此不用递归查找
	if service.IsAlias() {
		return service.Reference
	}

	return service.ID
}

// 根据服务名获取服务缓存数据
// 注意，如果是服务别名查询，这里会返回别名的源服务，不会返回别名
func (s *Server) getServiceCache(name string, namespace string) *model.Service {
	sc := s.caches.Service()
	service := sc.GetServiceByName(name, namespace)
	if service == nil {
		return nil
	}
	// 如果是服务别名，继续查找一下
	if service.IsAlias() {
		service = sc.GetServiceByID(service.Reference)
		if service == nil {
			return nil
		}
	}

	if service.Meta == nil {
		service.Meta = make(map[string]string)
	}
	return service
}

// GetRouterConfigWithCache User Client Get Service Routing Configuration Information
func (s *Server) GetRouterConfigWithCache(ctx context.Context, req *apiservice.Service) *apiservice.DiscoverResponse {
	if s.caches == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_ClientAPINotOpen, req)
	}
	if req == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_EmptyRequest, req)
	}

	if req.GetName().GetValue() == "" {
		return api.NewDiscoverRoutingResponse(apimodel.Code_InvalidServiceName, req)
	}
	if req.GetNamespace().GetValue() == "" {
		return api.NewDiscoverRoutingResponse(apimodel.Code_InvalidNamespaceName, req)
	}

	resp := api.NewDiscoverRoutingResponse(apimodel.Code_ExecuteSuccess, nil)
	resp.Service = &apiservice.Service{
		Name:      req.GetName(),
		Namespace: req.GetNamespace(),
	}

	// 先从缓存获取ServiceID，这里返回的是源服务
	svc := s.getServiceCache(req.GetName().GetValue(), req.GetNamespace().GetValue())
	if svc == nil {
		return api.NewDiscoverRoutingResponse(apimodel.Code_NotFoundService, req)
	}
	out, err := s.caches.RoutingConfig().GetRoutingConfigV2(svc.ID, svc.Name, svc.Namespace)
	if err != nil {
		log.Error("[Server][Service][Routing] discover routing v2", utils.ZapRequestIDByCtx(ctx), zap.Error(err))
		return api.NewDiscoverRoutingResponse(apimodel.Code_ExecuteException, req)
	}

	if out == nil {
		return resp
	}

	// // 获取路由数据，并对比revision
	// if out.GetRevision() == req.GetRevision() {
	// 	return apiv2.NewDiscoverRoutingResponse(api.DataNoChange, req)
	// }

	// 数据不一致，发生了改变
	// 数据格式转换，service只需要返回二元组与routing的revision
	revision := utils.NewV2Revision()
	resp.Service.Revision = &wrappers.StringValue{Value: revision}

	resp.Routing = &apitraffic.Routing{
		Rules:    out,
		Revision: utils.NewStringValue(revision),
	}
	return resp
}
