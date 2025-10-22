// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/host"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/manager"

	"github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
	loadbalancertypes "github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
)

type Cluster struct {
	lb        loadbalancertypes.LoadBalancer
	hosts     []loadbalancertypes.Host
	lbType    loadbalancertypes.LoadBalancerType
	lbContext context.Context
}

type Manager struct {
	name     string
	clusters sync.Map
	mux      sync.Mutex
	hosts    []loadbalancertypes.Host
}

func createClusterHosts(name string, endpoints []types.Endpoint) []loadbalancertypes.Host {
	hosts := make([]loadbalancertypes.Host, 0, len(endpoints))
	for _, server := range endpoints {
		host := host.BuildHost(name, server.Address, server.Port, 0)
		host.SetLabels(server.Labels) // set labels for lora and multi version
		hosts = append(hosts, host)
	}
	return hosts
}

func NewClusterManager(name string, endpoints []types.Endpoint) *Manager {
	manager := &Manager{
		name:  name,
		hosts: createClusterHosts(name, endpoints),
	}
	return manager
}

func (cm *Manager) GetCluster(ctx context.Context, lbType loadbalancertypes.LoadBalancerType) (*Cluster, error) {
	if cluster, ok := cm.clusters.Load(lbType); ok {
		if cl, ok := cluster.(*Cluster); ok {
			return cl, nil
		}
	}
	cm.mux.Lock()
	defer cm.mux.Unlock()

	// read again after lock
	if cluster, ok := cm.clusters.Load(lbType); ok {
		if cl, ok := cluster.(*Cluster); ok {
			return cl, nil
		}
	}

	// create a new cluster
	lb := manager.CreateLbByType(lbType, ctx, cm.hosts)
	if lb == nil {
		return nil, fmt.Errorf("fail CreateLbByType with type %+v", lbType)
	}
	cluster := &Cluster{
		lb:        lb,
		lbType:    lbType,
		lbContext: ctx,
		hosts:     cm.hosts,
	}

	cm.clusters.Store(lbType, cluster)
	return cluster, nil
}

func (cm *Manager) UpdateCluster(endpoints []types.Endpoint) {
	lbHosts := createClusterHosts(cm.name, endpoints)
	cm.clusters.Range(func(key, value any) bool {
		cl, ok := value.(*Cluster)
		if ok {
			cl.hosts = lbHosts
			cl.lb = manager.CreateLbByType(cl.lbType, cl.lbContext, lbHosts)
		}
		return true
	})
}

func (c *Cluster) ChooseHost(ctx context.Context) loadbalancertypes.Host {
	host := c.lb.ChooseHost(ctx)
	return host
}

func (c *Cluster) GetHostsNumber() int {
	return len(c.hosts)
}
