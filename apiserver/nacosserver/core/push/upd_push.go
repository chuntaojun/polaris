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

package push

import (
	"fmt"
	"net"
	"sync"

	"github.com/polarismesh/polaris/apiserver/nacosserver/core"
)

type UdpPushCenter struct {
	lock sync.RWMutex

	subscribers map[string]core.Subscriber
	// connectors namespace -> service -> Connectors
	connectors map[string]map[string]map[string]*Connector
}

func (p *UdpPushCenter) AddSubscriber(s core.Subscriber) {
	p.lock.Lock()
	defer p.lock.Unlock()

	id := fmt.Sprintf("%s:%d", s.AddrStr, s.Port)
	if _, ok := p.subscribers[id]; ok {
		return
	}

	p.subscribers[id] = s

	if _, ok := p.connectors[s.NamespaceId]; !ok {
		p.connectors[s.NamespaceId] = map[string]map[string]*Connector{}
	}
	if _, ok := p.connectors[s.NamespaceId][s.ServiceName]; !ok {
		p.connectors[s.NamespaceId][s.ServiceName] = map[string]*Connector{}
	}
	if _, ok := p.connectors[s.NamespaceId][s.ServiceName][id]; !ok {
		conn := NewConnector(s)
		p.connectors[s.NamespaceId][s.ServiceName][id] = conn
	}
}

func (p *UdpPushCenter) RemoveSubscriber(s core.Subscriber) {

}

func (p *UdpPushCenter) EnablePush(s core.Subscriber) bool {
	return true
}

func (p *UdpPushCenter) Push(d *core.PushData) {
	namespace := d.Service.Namespace
	service := d.Service.Name

	p.lock.RLock()
	defer p.lock.RUnlock()

	if _, ok := p.connectors[namespace]; !ok {
		return
	}
	if _, ok := p.connectors[namespace][service]; !ok {
		return
	}

	for _, conn := range p.connectors[namespace][service] {
		// step 1: 数据序列化为 json
		// step 2: 按需进行数据压缩
		conn.send(nil)
	}
}

func NewConnector(s core.Subscriber) *Connector {
	return &Connector{
		subscriber: s,
	}
}

type Connector struct {
	subscriber core.Subscriber
	conn       *net.UDPConn
	lastErr    error
}

func (c *Connector) connect() error {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.IP(c.subscriber.AddrStr),
		Port: c.subscriber.Port,
	})

	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Connector) send(data []byte) {

}

func (c *Connector) IsAlive() bool {
	return c.lastErr == nil
}
