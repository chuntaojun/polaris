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
	"net/http"

	"github.com/emicklei/go-restful/v3"
	httpcommon "github.com/polarismesh/polaris/apiserver/httpserver/http"
	"github.com/polarismesh/polaris/apiserver/nacosserver/core"
	"github.com/polarismesh/polaris/apiserver/nacosserver/model"
)

func (n *NacosV1Server) GetClientServer() (*restful.WebService, error) {
	ws := new(restful.WebService)
	ws.Path("/nacos/v1/ns").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	return nil, nil
}

func (n *NacosV1Server) addServiceAccess(ws *restful.WebService) {
	ws.Route(ws.GET("/service").To(n.GetService))
	ws.Route(ws.POST("/service").To(n.CreateService))
	ws.Route(ws.DELETE("/service").To(n.DeleteService))
	ws.Route(ws.PUT("/service").To(n.UpdateService))
	ws.Route(ws.GET("/service/list").To(n.ListServices))
}

func (n *NacosV1Server) addInstanceAccess(ws *restful.WebService) {
	ws.Route(ws.POST("/instance").To(n.RegisterInstance))
	ws.Route(ws.DELETE("/instance").To(n.DeRegisterInstance))
	ws.Route(ws.GET("/instance/list").To(n.ListInstances))
}

func (n *NacosV1Server) GetService(req *restful.Request, rsp *restful.Response) {

}

func (n *NacosV1Server) CreateService(req *restful.Request, rsp *restful.Response) {

}

func (n *NacosV1Server) DeleteService(req *restful.Request, rsp *restful.Response) {

}

func (n *NacosV1Server) UpdateService(req *restful.Request, rsp *restful.Response) {

}

func (n *NacosV1Server) ListServices(req *restful.Request, rsp *restful.Response) {

}

func (n *NacosV1Server) RegisterInstance(req *restful.Request, rsp *restful.Response) {
	handler := httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	ctx := handler.ParseHeaderContext()

	namespace := optional(req, model.ParamNamespaceID, model.DefaultNacosNamespace)
	ins, err := BuildInstance(namespace, req)
	if err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}

	if err := n.handleRegister(ctx, namespace, ins.ServiceName, ins); err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}
	core.WrirteSimpleResponse("ok", http.StatusOK, rsp)
}

func (n *NacosV1Server) DeRegisterInstance(req *restful.Request, rsp *restful.Response) {
	handler := httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	ctx := handler.ParseHeaderContext()

	namespace := optional(req, model.ParamNamespaceID, model.DefaultNacosNamespace)
	ins, err := BuildInstance(namespace, req)
	if err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}

	if err := n.handleDeregister(ctx, namespace, ins.ServiceName, ins); err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}
	core.WrirteSimpleResponse("ok", http.StatusOK, rsp)
}

func (n *NacosV1Server) Heartbeat(req *restful.Request, rsp *restful.Response) {
	handler := httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	ctx := handler.ParseHeaderContext()

	beat, err := BuildClientBeat(req)
	if err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}

	data, err := n.handleBeat(ctx, beat.Namespace, beat.ServiceName, beat)
	if err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}
	core.WrirteNacosResponse(data, rsp)
}

func (n *NacosV1Server) ListInstances(req *restful.Request, rsp *restful.Response) {
	handler := httpcommon.Handler{
		Request:  req,
		Response: rsp,
	}

	ctx := handler.ParseHeaderContext()

	data, err := n.handleQueryInstances(ctx, httpcommon.ParseQueryParams(req))
	if err != nil {
		core.WrirteNacosErrorResponse(err, rsp)
		return
	}
	core.WrirteNacosResponse(data, rsp)
}

func (n *NacosV1Server) ServerHealthStatus(req *restful.Request, rsp *restful.Response) {

}
