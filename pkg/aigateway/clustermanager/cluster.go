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

	"github.com/aigw-project/aigw/pkg/aigateway/clustermanager/cluster"
	"github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
	loadbalancertypes "github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
)

type Cluster struct {
	name    string
	manager *cluster.Manager
}

func newCluster(info types.ClusterInfo) *Cluster {
	cl := &Cluster{
		name: info.Name,
	}

	cl.manager = cluster.NewClusterManager(info.Name, info.Endpoints)

	return cl
}

func (c *Cluster) NextServer(ctx context.Context, lbType loadbalancertypes.LoadBalancerType) (loadbalancertypes.Host, error) {
	cm := c.manager

	cl, err := cm.GetCluster(ctx, lbType)
	if err != nil {
		return nil, fmt.Errorf("no cluster(%s) for lbType(%v), err: %+v ", c.name, lbType, err)
	}

	host := cl.ChooseHost(ctx)
	if host == nil {
		return nil, fmt.Errorf("no host in cluster(%s) for lbType(%v)", c.name, lbType)
	}

	return host, nil
}

func (c *Cluster) updateServers(info types.ClusterInfo) {
	c.manager.UpdateCluster(info.Endpoints)
}
