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

import (
	"errors"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type ClusterInfoProvider interface {
	GetClusterInfo(name string) (*ClusterInfo, error)
	WatchCluster(name string, notifier ClusterInfoNotifier)
	GetAllClusters() []*ClusterInfo
}

type BaseClusterInfoProvider struct {
	AllClusters map[string]*ClusterInfo
}

func (p *BaseClusterInfoProvider) GetAllClusters() []*ClusterInfo {
	clusters := make([]*ClusterInfo, 0, len(p.AllClusters))
	for _, cluster := range p.AllClusters {
		clusters = append(clusters, cluster)
	}
	return clusters
}

func (p *BaseClusterInfoProvider) getCluster(name string) *ClusterInfo {
	if cluster, ok := p.AllClusters[name]; ok {
		return cluster
	}
	api.LogErrorf("cluster %s not found, all clusters: %v", name, p.AllClusters)
	return nil
}

func (p *BaseClusterInfoProvider) GetClusterInfo(name string) (*ClusterInfo, error) {
	if cluster := p.getCluster(name); cluster != nil {
		return cluster, nil
	}
	return nil, errors.New("cluster not found")
}

func (p *BaseClusterInfoProvider) WatchCluster(name string, notifier ClusterInfoNotifier) {
	// TODO: static cluster won't change, so just notify once
	if cluster := p.getCluster(name); cluster != nil {
		notifier(cluster)
	}
}
