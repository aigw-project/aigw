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

package prom

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"mosn.io/htnn/api/pkg/filtermanager/api"
)

var (
	// AppVersionInfo is a prometheus metric that counts the version information about AIGW
	AppVersionInfo = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigw_version",
			Help: "Version information about AIGateway",
		},
		[]string{"version"},
	)

	// CdsMetricLock is a lock for cds metrics
	CdsMetricLock sync.Mutex

	// ClustersTotal is a prometheus metric that counts the number of watched clusters
	ClustersTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "watched_clusters_total",
			Help: "Number of watched clusters",
		})

	// ClusterEndpoints is a prometheus metric that counts the number of endpoints for each watched cluster
	ClusterEndpoints = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "watched_cluster_endpoints",
			Help: "Number of endpoints for each watched cluster",
		},
		[]string{"cluster"},
	)

	// ClusterLabels is a prometheus metric that counts the number of labels for each watched cluster endpoints
	ClusterLabels = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "watched_cluster_labels",
			Help: "Number of labels for in each cluster",
		},
		[]string{"cluster", "endpoint_address"},
	)

	// RuleMetricLock is a lock for rule metrics
	RuleMetricLock sync.Mutex

	// RuleTotal is a prometheus metric that counts the number of rules
	RuleTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aigw_rule_total",
			Help: "Number of rules",
		})

	// LbConfigTotal is a prometheus metric that counts the number of lb configs
	LbConfigTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aigw_lbconfig_total",
			Help: "Number of lbconfigs",
		})

	// CircuitBreakerState is a prometheus metric that counts the state of circuit breaker for each host
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aigw_servicediscovery_circuitbreaker_state",
			Help: "The current state of the circuit breaker for each host. 0: Closed, 1: Half-Open, 2: Open.",
		},
		[]string{"host"},
	)

	// MetacenterRequestDuration is a prometheus metric that counts the duration of requests to aigwmetacenter
	MetacenterRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "metacenter_http_request_duration_us",
			Help: "Histogram of HTTP request durations to Metacenter APIs",
			// 桶分布[0.1ms, 0.2ms, 0.4ms, 0.8ms, 1.6ms, 3.2ms, 6.4ms, 12.8ms, 25.6ms, 51.2ms, 102.4ms, 204.8ms, 409.6ms]
			Buckets: prometheus.ExponentialBuckets(
				100,
				2.0,
				12,
			),
		},
		[]string{"instance", "method", "path"},
	)

	// MetacenterRequestsTotal is a prometheus metric that counts the total number of requests to aigwmetacenter
	MetacenterRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "metacenter_http_requests_total",
			Help: "Total number of HTTP requests to Metacenter",
		},
		[]string{"instance", "method", "path"},
	)
)

func UpdateBreakerState(host, currentState string) {
	var numericState float64
	switch currentState {
	case "closed":
		numericState = 0
	case "half-open":
		numericState = 1
	case "open":
		numericState = 2
	default:
		numericState = -1
	}

	api.LogInfof("circuit breaker state %s for host %s", currentState, host)
	CircuitBreakerState.WithLabelValues(host).Set(numericState)
}
