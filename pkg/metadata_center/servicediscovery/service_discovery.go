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

package servicediscovery

import (
	"hash/fnv"
	"log"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/circuitbreaker"
	pkgcommon "github.com/aigw-project/aigw/pkg/common"
	"github.com/aigw-project/aigw/pkg/metadata_center/types"
	"github.com/aigw-project/aigw/pkg/prom"
)

const (
	AigwMetaDataCenter_Host                     = "AIGW_META_DATA_CENTER_HOST"
	AigwMetaDataCenter_Port                     = "AIGW_META_DATA_CENTER_PORT"
	AigwMetaDataCenter_ColdStartDelay           = "AIGW_META_COLD_START_DELAY"
	AigwMetaDataCenter_DnsLookUpInterval        = "AIGW_META_DATA_CENTER_DNS_LOOKUP_INTERVAL"
	AigwMetaDataCenter_EndpointFailureThreshold = "AIGW_METADATA_CENTER_ENDPOINT_MAX_FAILURES"
	AigwMetaDataCenter_EndpointCooldownPeriod   = "AIGW_METADATA_CENTER_ENDPOINT_COOLDOWN_SECONDS"
	AigwMetaDataCenter_EndpointHalfOpenRequests = "AIGW_METADATA_CENTER_ENDPOINT_HALF_OPEN_REQUESTS"
)

type Config struct {
	Host                     string
	Port                     int
	LookupInterval           time.Duration
	ColdStartDelay           time.Duration
	EndpointFailureThreshold int
	EndpointCooldownPeriod   time.Duration
	EndpointHalfOpenRequests int
}

type nodeStatus struct {
	discoveredAt time.Time
	breaker      circuitbreaker.CircuitBreaker
}

type ServiceDiscovery struct {
	config  Config
	mutex   sync.RWMutex
	nodeMap map[string]*nodeStatus
	stopCh  chan struct{}
}

func NewSimpleService(config Config) *ServiceDiscovery {
	svc := &ServiceDiscovery{
		config:  config,
		nodeMap: make(map[string]*nodeStatus),
		stopCh:  make(chan struct{}),
	}

	host := config.Host
	if host == "" {
		log.Printf("metadata center host is empty, service discovery not started")
		return svc
	}

	// parse host if it's an IP address
	if net.ParseIP(host) != nil {
		svc.initStaticHost()
	} else {
		svc.StartDNSLoop()
	}
	return svc
}

func (sd *ServiceDiscovery) initStaticHost() {
	log.Printf("using static ip %s as metadata center host", sd.config.Host)

	sd.nodeMap[sd.config.Host] = &nodeStatus{
		discoveredAt: time.Now(),
		breaker: circuitbreaker.NewCircuitBreaker(circuitbreaker.CircuitBreakerConfig{
			MaxFailures:      sd.config.EndpointFailureThreshold,
			CooldownPeriod:   sd.config.EndpointCooldownPeriod,
			HalfOpenRequests: sd.config.EndpointHalfOpenRequests,
		}),
	}
}

func (sd *ServiceDiscovery) StartDNSLoop() {
	ticker := time.NewTicker(sd.config.LookupInterval)
	log.Printf("DNSLookUp loop started, interval: %s", sd.config.LookupInterval)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recovered from panic, panic in DnsLookUp goroutine")
			}
			ticker.Stop()
			log.Printf("DnsLookUp goroutine exited")
		}()

		for {
			select {
			case <-ticker.C:
				sd.dnsLookUp()
			case <-sd.stopCh:
				log.Printf("DNSLookUp loop received stop signal, shutting down.")
				return
			}
		}
	}()
}

func (sd *ServiceDiscovery) Shutdown() {
	close(sd.stopCh)
}

func (sd *ServiceDiscovery) dnsLookUp() {
	hosts, err := net.LookupIP(sd.config.Host)
	if err != nil {
		api.LogErrorf("LookupIP failed for domain '%s': %v", sd.config.Host, err)
		return
	}

	if len(hosts) == 0 {
		api.LogErrorf("LookupIP for domain '%s' returned no hosts", sd.config.Host)
	}

	api.LogDebugf("DNS lookup for domain '%s' found hosts: %v", sd.config.Host, hosts)

	config := circuitbreaker.CircuitBreakerConfig{
		MaxFailures:      sd.config.EndpointFailureThreshold,
		CooldownPeriod:   sd.config.EndpointCooldownPeriod,
		HalfOpenRequests: sd.config.EndpointHalfOpenRequests,
	}

	sd.mutex.Lock()
	defer sd.mutex.Unlock()

	newHosts := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		hostStr := host.String()
		newHosts[hostStr] = struct{}{}
		if _, ok := sd.nodeMap[hostStr]; !ok {
			sd.nodeMap[hostStr] = &nodeStatus{
				discoveredAt: time.Now(),
				breaker:      circuitbreaker.NewCircuitBreaker(config),
			}
		}

		prom.UpdateBreakerState(hostStr, sd.nodeMap[hostStr].breaker.State())
	}

	for host := range sd.nodeMap {
		if _, ok := newHosts[host]; !ok {
			delete(sd.nodeMap, host)
			prom.CircuitBreakerState.DeleteLabelValues(host)
			api.LogInfof("Node removed for domain '%s': %s, removed breaker metrics", sd.config.Host, host)
		}
	}
}

func (sd *ServiceDiscovery) GetAvailableHosts() []string {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()

	var warmHosts []string
	var coldHosts []string
	now := time.Now()

	for host, node := range sd.nodeMap {
		api.LogDebugf("Host %s for domain '%s' state: %s", host, sd.config.Host, node.breaker.State())
		if !node.breaker.Allow() {
			api.LogDebugf("Host %s for domain '%s' is skipped due to open circuit breaker.", host, sd.config.Host)
			continue
		}

		isCold := sd.config.ColdStartDelay > 0 && now.Sub(node.discoveredAt) < sd.config.ColdStartDelay
		if isCold {
			coldHosts = append(coldHosts, host)
		} else {
			warmHosts = append(warmHosts, host)
		}
	}

	if len(warmHosts) > 0 {
		api.LogDebugf("GetAvailableHosts for domain '%s' warm hosts result: %v, cold hosts result: %v", sd.config.Host, warmHosts, coldHosts)
		return warmHosts
	}

	api.LogDebugf("No warm hosts available for domain '%s', falling back to cold hosts: %v", sd.config.Host, coldHosts)
	return coldHosts
}

func (sd *ServiceDiscovery) GetHosts(key string, num int) []string {
	hosts := sd.GetAvailableHosts()
	if len(hosts) == 0 || num <= 0 {
		return nil
	}

	hostLen := len(hosts)
	n := min(hostLen, num)

	// TODO: optimize: avoid sorting every time
	sort.Strings(hosts)

	selected := make([]string, 0, n)
	idx := int(hashKey(key) % uint32(hostLen))
	for i := 0; i < n; i++ {
		hosts = append(hosts, hosts[(idx+i)%hostLen])
	}

	return selected
}

func hashKey(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func (sd *ServiceDiscovery) GetPort() int {
	return sd.config.Port
}

func (sd *ServiceDiscovery) ReportSuccess(host string) {
	if node := sd.getNode(host); node != nil {
		node.breaker.RecordSuccess()
	}
}

func (sd *ServiceDiscovery) ReportFailure(host string) {
	if node := sd.getNode(host); node != nil {
		node.breaker.RecordFailure()
	}
}

func (sd *ServiceDiscovery) State(host string) string {
	if node := sd.getNode(host); node != nil {
		return node.breaker.State()
	}

	return ""
}

func (sd *ServiceDiscovery) getNode(host string) *nodeStatus {
	sd.mutex.RLock()
	defer sd.mutex.RUnlock()
	return sd.nodeMap[host]
}

var (
	createOnce sync.Once
	service    *ServiceDiscovery
)

// CreateSimpleService creates a singleton ServiceDiscovery with config from environment variables
// AIGW_META_DATA_CENTER_HOST could be an IP or domain name.
func CreateSimpleService() types.Service {
	createOnce.Do(func() {
		log.Printf("init metadata center service discovery")
		config := Config{
			Host:                     os.Getenv(AigwMetaDataCenter_Host),
			ColdStartDelay:           pkgcommon.GetDurationFromEnv(AigwMetaDataCenter_ColdStartDelay, 10*time.Minute),
			LookupInterval:           pkgcommon.GetDurationFromEnv(AigwMetaDataCenter_DnsLookUpInterval, 5*time.Second),
			EndpointFailureThreshold: pkgcommon.GetIntFromEnv(AigwMetaDataCenter_EndpointFailureThreshold, 10),
			EndpointCooldownPeriod:   pkgcommon.GetDurationFromEnv(AigwMetaDataCenter_EndpointCooldownPeriod, 5*time.Second),
			EndpointHalfOpenRequests: pkgcommon.GetIntFromEnv(AigwMetaDataCenter_EndpointHalfOpenRequests, 3),
			Port:                     pkgcommon.GetIntFromEnv(AigwMetaDataCenter_Port, 80),
		}

		service = NewSimpleService(config)
		log.Printf("metadata center service discovery started")
	})
	return service
}
