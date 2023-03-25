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
	nacosmodel "github.com/polarismesh/polaris/apiserver/nacosserver/model"
	"github.com/polarismesh/polaris/common/model"
)

type PushCenter interface {
	AddSubscriber(s Subscriber)
	RemoveSubscriber(s Subscriber)
	EnablePush(s Subscriber) bool
	Push(d *PushData)
}

type PushData struct {
	Service     *model.Service
	ServiceInfo *nacosmodel.ServiceInfo
}

type Subscriber struct {
	AddrStr     string
	Agent       string
	App         string
	Ip          string
	Port        int
	NamespaceId string
	ServiceName string
	Cluster     string
	Type        string
}

type PushCenterDelegate struct {
	udpPush  PushCenter
	grpcPush PushCenter
}
