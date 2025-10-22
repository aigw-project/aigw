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

package clustermanager

import (
	"context"
	"fmt"
	"sync"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	managertypes "github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
	loadbalancertypes "github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
)

type Manager struct {
	clusters            sync.Map
	mux                 sync.RWMutex
	clusterInfoProvider managertypes.ClusterInfoProvider
}

func NewClusterManager(provider managertypes.ClusterInfoProvider) loadbalancertypes.GlobalLoadBalancer {
	manager := &Manager{
		clusterInfoProvider: provider,
	}
	return manager
}

func (m *Manager) ChooseHost(ctx context.Context, clusterName string, lbType loadbalancertypes.LoadBalancerType) (loadbalancertypes.Host, error) {
	if v, ok := m.clusters.Load(clusterName); ok {
		if c, ok := v.(*Cluster); ok {
			api.LogDebugf("aiProxy load cluster name: %s", clusterName)
			return c.NextServer(ctx, lbType)
		}
	}
	m.mux.Lock()
	defer m.mux.Unlock()
	// get again after lock
	if v, ok := m.clusters.Load(clusterName); ok {
		if c, ok := v.(*Cluster); ok {
			api.LogDebugf("aiProxy load cluster name: %s after lock", clusterName)
			return c.NextServer(ctx, lbType)
		}
	}

	// create a new cluster
	clusterInfo, err := m.clusterInfoProvider.GetClusterInfo(clusterName)
	if err != nil || len(clusterInfo.Endpoints) == 0 {
		return nil, fmt.Errorf("%s: %v", clusterName, err)
	}

	cl := newCluster(*clusterInfo)
	api.LogDebugf("aiProxy new cluster, cluster info(%+v), cluster(%+v)", clusterInfo, cl)

	// watch cluster info
	m.clusterInfoProvider.WatchCluster(clusterName, m.updateServers)

	m.clusters.Store(clusterName, cl)
	return cl.NextServer(ctx, lbType)
}

func (m *Manager) updateServers(info managertypes.ClusterInfo) {
	v, ok := m.clusters.Load(info.Name)
	if ok {
		cl, ok := v.(*Cluster)
		if ok {
			cl.updateServers(info)
			return
		}
		// log unexpected type
	}
	// log unexpected cluster name
}
