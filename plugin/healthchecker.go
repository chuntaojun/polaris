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

package plugin

import (
	"os"
	"sync"
)

type CheckerPeer struct {
	Host string
	ID   string
}

// ReportRequest report heartbeat request
type ReportRequest struct {
	QueryRequest
	LocalHost  string
	CurTimeSec int64
	Count      int64
}

// CheckRequest check heartbeat request
type CheckRequest struct {
	QueryRequest
	ExpireDurationSec uint32
	CurTimeSec        func() int64
}

// CheckResponse check heartbeat response
type CheckResponse struct {
	Healthy              bool
	LastHeartbeatTimeSec int64
	StayUnchanged        bool
	Regular              bool
}

// QueryRequest query heartbeat request
type QueryRequest struct {
	InstanceId string
	Host       string
	Port       uint32
	Healthy    bool
}

// QueryResponse query heartbeat response
type QueryResponse struct {
	Server           string
	Exists           bool
	LastHeartbeatSec int64
	Count            int64
}

// AddCheckRequest add check request
type AddCheckRequest struct {
	Instances []string
	LocalHost string
}

// HealthCheckType health check type
type HealthCheckType int32

const (
	HealthCheckerHeartbeat HealthCheckType = iota + 1
)

var (
	healthCheckOnce = &sync.Once{}
)

// HealthChecker health checker plugin interface
type HealthChecker interface {
	Plugin
	// SetCheckerPeers set checker peer list
	SetCheckerPeers(peers []CheckerPeer)
	// Type for health check plugin, only one same type plugin is allowed
	Type() HealthCheckType
	// Report process heartbeat info report
	Report(request *ReportRequest) error
	// Check process the instance check
	Check(request *CheckRequest) (*CheckResponse, error)
	// Query queries the heartbeat time
	Query(request *QueryRequest) (*QueryResponse, error)
	// AddToCheck add the instances to check procedure
	AddToCheck(request *AddCheckRequest) error
	// RemoveFromCheck removes the instances from check procedure
	RemoveFromCheck(request *AddCheckRequest) error
	// Suspend health checker for entire expired duration manually
	Suspend()
	// SuspendTimeSec get the suspend time in seconds
	SuspendTimeSec() int64
	// Delete delete the id
	Delete(id string) error
	// DebugHandlers return debug handlers
	DebugHandlers() []DebugHandler
}

// GetHealthChecker get the health checker by name
func GetHealthChecker(name string, cfg *ConfigEntry) HealthChecker {
	plugin, exist := pluginSet[name]
	if !exist {
		return nil
	}

	healthCheckOnce.Do(func() {
		if err := plugin.Initialize(cfg); err != nil {
			log.Errorf("HealthChecker plugin init err: %s", err.Error())
			os.Exit(-1)
		}
	})

	return plugin.(HealthChecker)
}
