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

package manager

import (
	"context"
	"sync"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
)

var lbFactories map[types.LoadBalancerType]func(context.Context, []types.Host) types.LoadBalancer
var factoryMutex sync.Mutex

func RegisterLbType(lbType types.LoadBalancerType, f func(context.Context, []types.Host) types.LoadBalancer) {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()
	if lbFactories == nil {
		lbFactories = make(map[types.LoadBalancerType]func(context.Context, []types.Host) types.LoadBalancer)
	}
	lbFactories[lbType] = f
}

func CreateLbByType(lbType types.LoadBalancerType, context context.Context, hosts []types.Host) types.LoadBalancer {
	if f, ok := lbFactories[lbType]; ok {
		return f(context, hosts)
	}

	// RoundRobin as default
	if f, ok := lbFactories[types.RoundRobin]; ok {
		return f(context, hosts)
	}

	return nil
}
