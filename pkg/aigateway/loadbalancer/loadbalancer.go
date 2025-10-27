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

package loadbalancer

import (
	"context"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/inferencelb"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
)

func ChooseServer(ctx context.Context, callbacks api.FilterCallbackHandler, header api.HeaderMap, cluster string, lbType types.LoadBalancerType) (types.Host, error) {
	ctx = context.WithValue(ctx, inferencelb.KeyClusterName, cluster)
	host, err := globalLoadBalancer.ChooseHost(ctx, cluster, lbType)
	if err != nil {
		return nil, err
	}

	header.Set("Cluster-Name", cluster)
	callbacks.RefreshRouteCache()

	if err = callbacks.DecoderFilterCallbacks().SetUpstreamOverrideHost(host.Address(), false); err != nil {
		return nil, err
	}

	return host, nil
}
