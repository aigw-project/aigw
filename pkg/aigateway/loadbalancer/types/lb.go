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

package types

import "context"

type LoadBalancerType string

const (
	Random     LoadBalancerType = "Random"
	RoundRobin LoadBalancerType = "RoundRobin"
)

type Host interface {
	Ip() string
	Port() uint32
	// Address returns the address in "ip:port" format
	Address() string
	Labels() map[string]string
}

// GlobalLoadBalancer choose host from multiple clusters
type GlobalLoadBalancer interface {
	ChooseHost(context context.Context, cluster string, lbType LoadBalancerType) (Host, error)
}

// LoadBalancer choose host from single cluster
type LoadBalancer interface {
	ChooseHost(context context.Context) Host
}
