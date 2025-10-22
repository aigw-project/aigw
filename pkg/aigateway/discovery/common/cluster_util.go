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

package common

import (
	"time"

	clustercfg "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corecfg "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointcfg "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
)

// GenerateCluster create cds cluster from endpoints
func GenerateCluster(name string, endpoints []types.Endpoint, grpc bool) *clustercfg.Cluster {
	lbEndpoints := make([]*endpointcfg.LbEndpoint, 0, len(endpoints))
	for _, e := range endpoints {
		lbEndpoints = append(lbEndpoints, &endpointcfg.LbEndpoint{
			HostIdentifier: &endpointcfg.LbEndpoint_Endpoint{
				Endpoint: &endpointcfg.Endpoint{
					Address: &corecfg.Address{
						Address: &corecfg.Address_SocketAddress{
							SocketAddress: &corecfg.SocketAddress{
								Protocol: corecfg.SocketAddress_TCP,
								Address:  e.Address,
								PortSpecifier: &corecfg.SocketAddress_PortValue{
									PortValue: e.Port,
								},
							},
						},
					},
				},
			},
			Metadata: &corecfg.Metadata{
				FilterMetadata: map[string]*structpb.Struct{
					"envoy.lb": {
						Fields: map[string]*structpb.Value{
							"address": structpb.NewStringValue(e.Address),
						},
					},
				},
			},
		})
	}

	defaultConcurrency := uint32(1024 * 1024)

	http2Config := &corecfg.Http2ProtocolOptions{}
	if !grpc {
		http2Config = nil
	}

	return &clustercfg.Cluster{
		Name:                 name,
		ConnectTimeout:       durationpb.New(2 * time.Second),
		Http2ProtocolOptions: http2Config,
		CircuitBreakers: &clustercfg.CircuitBreakers{
			Thresholds: []*clustercfg.CircuitBreakers_Thresholds{
				{
					Priority:       corecfg.RoutingPriority_DEFAULT,
					MaxConnections: wrapperspb.UInt32(defaultConcurrency),
					MaxRequests:    wrapperspb.UInt32(defaultConcurrency),
				},
			},
		},
		LoadAssignment: &endpointcfg.ClusterLoadAssignment{
			ClusterName: name,
			Endpoints: []*endpointcfg.LocalityLbEndpoints{
				{
					LbEndpoints: lbEndpoints,
				},
			},
		},
		LbSubsetConfig: &clustercfg.Cluster_LbSubsetConfig{
			FallbackPolicy: clustercfg.Cluster_LbSubsetConfig_ANY_ENDPOINT,
			SubsetSelectors: []*clustercfg.Cluster_LbSubsetConfig_LbSubsetSelector{
				{
					Keys: []string{"address"},
				},
			},
		},
	}
}

// GenerateDeltaDiscoveryResponse is a helper function that creates a DeltaDiscoveryResponse
// from either a single discovery.Resource or a slice of them.
func GenerateDeltaDiscoveryResponse(typeURL string, nonce string, input ...*discovery.Resource) *discovery.DeltaDiscoveryResponse {
	resources := make([]*discovery.Resource, 0, len(input))
	resources = append(resources, input...)

	return &discovery.DeltaDiscoveryResponse{
		Resources: resources,
		Nonce:     nonce,
		TypeUrl:   typeURL,
	}
}

// GenerateDeltaDiscoveryResponseWithRemovedResources likes GenerateDeltaDiscoveryResponse but with removed resources
func GenerateDeltaDiscoveryResponseWithRemovedResources(typeURL string, nonce string, input []*discovery.Resource, removed []string) *discovery.DeltaDiscoveryResponse {
	resp := &discovery.DeltaDiscoveryResponse{
		Nonce:   nonce,
		TypeUrl: typeURL,
	}

	if len(input) > 0 {
		resources := make([]*discovery.Resource, 0, len(input))
		resources = append(resources, input...)
		resp.Resources = resources
	}

	if len(removed) > 0 {
		resp.RemovedResources = removed
	}

	return resp
}

// GenerateNonce generate nonce from uuid. the istio use server version + uuid
func GenerateNonce() string {
	return uuid.New().String()
}
