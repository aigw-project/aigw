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
	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	managertypes "github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/dynamic_provider"
	"github.com/aigw-project/aigw/pkg/aigateway/discovery/static_provider"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer"
	"github.com/aigw-project/aigw/pkg/common"
)

func init() {
	providerType := common.GetStrFromEnv("AIGW_CLUSTER_PROVIDER_TYPE", "static")
	var clusterProvider managertypes.ClusterInfoProvider
	if providerType == "static" {
		clusterProvider = static_provider.NewStaticClusterProvider()
	} else {
		clusterProvider = dynamic_provider.NewDynamicClusterProvider()
	}
	lb := NewClusterManager(clusterProvider)

	api.LogInfof("registering cluster manager as global load balancer")
	loadbalancer.RegisterGlobalLoadBalancer(lb)
}
