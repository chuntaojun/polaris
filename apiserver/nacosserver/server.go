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

package nacosserver

import (
	"context"

	"github.com/polarismesh/polaris/apiserver"
)

const (
	ProtooclName = "nacos"
)

type Server interface {
	// Initialize API初始化逻辑
	Initialize(ctx context.Context, basePort uint32, option map[string]interface{}) error
	// Serve 开始运行服务
	Serve(errCh chan error)
	// Stop 停止API端口监听
	Stop()
}

type NacosServer struct {
	listenIP   string
	listenPort uint32

	servers []Server
}

// GetProtocol API协议名
func (n *NacosServer) GetProtocol() string {
	return ProtooclName
}

// GetPort API的监听端口
func (n *NacosServer) GetPort() uint32 {
	return n.listenPort
}

// Initialize API初始化逻辑
func (n *NacosServer) Initialize(ctx context.Context, option map[string]interface{}, api map[string]apiserver.APIConfig) error {
	for i := range n.servers {
		if err := n.servers[i].Initialize(ctx, n.GetPort(), option); err != nil {
			return err
		}
	}
	return nil
}

// Run API服务的主逻辑循环
func (n *NacosServer) Run(errCh chan error) {

}

// Stop 停止API端口监听
func (n *NacosServer) Stop() {

}

// Restart 重启API
func (n *NacosServer) Restart(option map[string]interface{}, api map[string]apiserver.APIConfig, errCh chan error) error {
	// TODO not support
	return nil
}
