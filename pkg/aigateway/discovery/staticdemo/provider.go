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

package staticdemo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	discoverypb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"
	"mosn.io/htnn/api/pkg/filtermanager/api"

	managertypes "github.com/aigw-project/aigw/pkg/aigateway/clustermanager/types"
)

const (
	staticClusterFile = "/etc/aigw/static_clusters.json"

	// istio
	istioPodAddrEnv = "AIGW_ISTIO_ADDR"
	defaultAddr     = "istiod.istio-system.svc.cluster.local:15010"
	ipv4PortRe      = `^((?:\d{1,3}\.){3}\d{1,3}):(\d{1,5})$`
	defaultNodeId   = "router~127.0.0.1~aigw.default~cluster.local"
)

type StaticEndpoint struct {
	Address string `json:"address"`
	Port    uint32 `json:"port"`
}

type StaticCluster struct {
	Name      string           `json:"name"`
	Endpoints []StaticEndpoint `json:"endpoints"`
}

type ClustersConfig struct {
	Clusters []StaticCluster `json:"clusters"`
}

var config ClustersConfig
var istioXdsAddr string

func init() {
	fp, err := os.Open(staticClusterFile)
	if err != nil {
		api.LogErrorf("failed to open %s: %v", staticClusterFile, err)
		return
	}
	defer fp.Close()

	data, err := io.ReadAll(fp)
	if err != nil {
		api.LogErrorf("failed to read %s: %v", staticClusterFile, err)
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		api.LogErrorf("failed to unmarshal %s: %v", staticClusterFile, err)
	}

	api.LogInfof("static cluster config loaded: %+v", config)

	// load istio pod addr
	istioXdsAddr = resolveIstioAddr()
}

// get istio xds server ip:port from enviroment variable; use defaultNodeId by default
func resolveIstioAddr() string {
	raw := os.Getenv(istioPodAddrEnv)
	if raw == "" {
		api.LogInfof("set istio xds addr, env %s not set, use default %s\n", istioPodAddrEnv, defaultAddr)
		return defaultAddr
	}

	ip, port, err := net.SplitHostPort(raw)
	if err != nil {
		api.LogWarnf("set istio xds addr, env %s format invalid: %s, use default %s\n", istioPodAddrEnv, raw, defaultAddr)
		return defaultAddr
	}

	addr := fmt.Sprintf("%s:%s", ip, port)
	api.LogInfof("set istio xds addr, env %s valid, use %s\n", istioPodAddrEnv, addr)
	return addr
}

type StaticClusterProvider struct {
	allClusters map[string]*managertypes.ClusterInfo
}

func NewStaticClusterProvider() managertypes.ClusterInfoProvider {
	p := &StaticClusterProvider{
		allClusters: make(map[string]*managertypes.ClusterInfo),
	}
	for _, c := range config.Clusters {
		endpoints := make([]managertypes.Endpoint, 0, len(c.Endpoints))
		for _, ep := range c.Endpoints {
			endpoints = append(endpoints, managertypes.Endpoint{
				Address: ep.Address,
				Port:    ep.Port,
			})
		}
		p.allClusters[c.Name] = &managertypes.ClusterInfo{
			Name:      c.Name,
			Endpoints: endpoints,
		}
	}

	p.AutoUpdateFromPilot(defaultNodeId, 10*time.Second)

	api.LogInfof("new static cluster provider: %+v", p)

	startCdsServer(defaultCdsAddress, p)
	return p
}

func extractClusterNames(resources []*anypb.Any) []string {
	var names []string
	for _, res := range resources {
		var c clusterpb.Cluster
		if err := res.UnmarshalTo(&c); err != nil {
			api.LogErrorf("unmarshal cluster failed: %v", err)
			continue
		}
		names = append(names, c.Name)
	}
	return names
}

// extract endpoint info from ClusterLoadAssignment
func extractFromCLA(cla *endpointpb.ClusterLoadAssignment) []managertypes.Endpoint {
	var eps []managertypes.Endpoint
	for _, locality := range cla.Endpoints {
		for _, lb := range locality.LbEndpoints {
			sock := lb.GetEndpoint().GetAddress().GetSocketAddress()
			if sock == nil {
				continue
			}
			ep := managertypes.Endpoint{
				Address: sock.Address,
				Port:    sock.PortSpecifier.(*corepb.SocketAddress_PortValue).PortValue,
				Labels:  make(map[string]string),
			}
			if meta := lb.GetMetadata(); meta != nil {
				for k, v := range meta.GetFilterMetadata() {
					if k == "istio" {
						for kk, vv := range v.GetFields() {
							ep.Labels[kk] = vv.GetStringValue()
						}
					}
				}
			}
			eps = append(eps, ep)
		}
	}
	return eps
}

// subscribe istio pilot and pull the cds info
func (p *StaticClusterProvider) subscribeIstioPilot(nodeID string) error {
	conn, err := grpc.NewClient(
		istioXdsAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := discoverypb.NewAggregatedDiscoveryServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		return err
	}

	// get cds
	cdsReq := &discoverypb.DiscoveryRequest{
		Node:    &corepb.Node{Id: nodeID},
		TypeUrl: "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	}
	if err := stream.Send(cdsReq); err != nil {
		return err
	}
	cdsResp, err := stream.Recv()
	if err != nil {
		return err
	}
	clusterNames := extractClusterNames(cdsResp.Resources)
	if len(clusterNames) == 0 {
		return nil
	}

	// get eds
	edsReq := &discoverypb.DiscoveryRequest{
		Node:          &corepb.Node{Id: nodeID},
		TypeUrl:       "type.googleapis.com/envoy.config.endpoint.v3.ClusterLoadAssignment",
		ResourceNames: clusterNames,
	}
	if err := stream.Send(edsReq); err != nil {
		return err
	}
	edsResp, err := stream.Recv()
	if err != nil {
		return err
	}

	// fill eds info to provider
	for _, res := range edsResp.Resources {
		var cla endpointpb.ClusterLoadAssignment
		if err := res.UnmarshalTo(&cla); err != nil {
			api.LogErrorf("unmarshal cla failed: %v", err)
			continue
		}
		p.allClusters[cla.ClusterName] = &managertypes.ClusterInfo{
			Name:      cla.ClusterName,
			Endpoints: extractFromCLA(&cla),
		}
	}
	return nil
}

// updata the snapshot form istio
func (p *StaticClusterProvider) AutoUpdateFromPilot(nodeID string, interval time.Duration) {
	go func() {
		for {
			err := p.subscribeIstioPilot(nodeID)
			if err != nil {
				api.LogErrorf("failed to pull from pilot: %v", err)
			}
			time.Sleep(interval)
		}
	}()
}

func (p *StaticClusterProvider) GetAllClusters() []*managertypes.ClusterInfo {
	clusters := make([]*managertypes.ClusterInfo, 0, len(p.allClusters))
	for _, cluster := range p.allClusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

func (p *StaticClusterProvider) getCluster(name string) *managertypes.ClusterInfo {
	if cluster, ok := p.allClusters[name]; ok {
		return cluster
	}
	api.LogErrorf("cluster %s not found, all clusters: %v", name, p.allClusters)
	return nil
}

func (p *StaticClusterProvider) GetClusterInfo(name string) (*managertypes.ClusterInfo, error) {
	if cluster := p.getCluster(name); cluster != nil {
		return cluster, nil
	}
	return nil, errors.New("cluster not found")
}

func (p *StaticClusterProvider) WatchCluster(name string, notifier managertypes.ClusterInfoNotifier) {
	// TODO: static cluster won't change, so just notify once
	if cluster := p.getCluster(name); cluster != nil {
		notifier(cluster)
	}
}
