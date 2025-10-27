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

package inferencelb

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	filtermanager "mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/manager"
	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/types"
	pkgcommon "github.com/aigw-project/aigw/pkg/common"
	"github.com/aigw-project/aigw/pkg/metadata_center"
	mctypes "github.com/aigw-project/aigw/pkg/metadata_center/types"
	"github.com/aigw-project/aigw/pkg/request"
)

const (
	InferenceLB types.LoadBalancerType = "inference_lb"
)

func init() {
	manager.RegisterLbType(InferenceLB, InferenceLoadBalancerFactory)
}

const (
	KeyFilterCallback pkgcommon.LBCtxKey = "lb.filterCallback"
	KeyClusterName    pkgcommon.LBCtxKey = "lb.clusterName"
	KeyBackendName    pkgcommon.LBCtxKey = "lb.backendName"
	KeyModelName      pkgcommon.LBCtxKey = "lb.modelName"
	KeyTraceId        pkgcommon.LBCtxKey = "lb.traceId"
	KeyPromptHash     pkgcommon.LBCtxKey = "lb.promptHash"
	KeyHostMatchInfo  pkgcommon.LBCtxKey = "lb.hostMatchInfo"
	KeyLbSelector     pkgcommon.LBCtxKey = "lb.selector"

	KeyLoadAwareEnable   pkgcommon.LBCtxKey = "lb.load_aware_enable"
	KeyCacheAwareEnable  pkgcommon.LBCtxKey = "lb.cache_aware_enable"
	KeyCandidatePercent  pkgcommon.LBCtxKey = "lb.candidate_percent"
	KeyCacheRatioWeight  pkgcommon.LBCtxKey = "lb.cache_ratio_weight"
	KeyLoadRequestWeight pkgcommon.LBCtxKey = "lb.request_load_weight"
	KeyLoadPrefillWeight pkgcommon.LBCtxKey = "lb.prefill_load_weight"

	KeyCacheDuration = "cache_duration"
	KeyUseMetaCache  = "use_cache"
	KeyLoadDuration  = "load_duration"
	KeyUseMetaLoad   = "use_load"

	// factor weights default value
	InferLbCacheRatioWeight  = 2
	InferLbRequestLoadWeight = 1
	InferLbPrefillLoadWeight = 3
)

type inferenceLoadBalancer struct {
	hosts []types.Host
}

func InferenceLoadBalancerFactory(context context.Context, hosts []types.Host) types.LoadBalancer {
	return &inferenceLoadBalancer{
		hosts: hosts,
	}
}

func (lb *inferenceLoadBalancer) ChooseHost(ctx context.Context) types.Host {
	candidateHosts := lb.hosts
	selector := pkgcommon.GetValueFromCtx(ctx, KeyLbSelector, map[string]string{})
	if len(selector) > 0 {
		candidateHosts = filterHostsBySelector(lb.hosts, selector)
		api.LogDebugf("filter hosts by selector: %v", candidateHosts)
	}

	if len(candidateHosts) == 0 {
		return nil
	}

	clusterName := pkgcommon.MustGetValueFromCtx[string](ctx, KeyClusterName)
	traceId := pkgcommon.GetValueFromCtx(ctx, KeyTraceId, "")
	ctx = context.WithValue(ctx, metadata_center.MetaCenterTraceId, traceId)

	// only use random when cluster's load-aware is set to false, default is true when not set
	hosts := candidateHosts
	if isModelLoadAwareEnable(ctx) {
		candNum := candidateNumFromContext(ctx, candidateHosts)
		hosts = lb.GetCandidateByStats(ctx, clusterName, candidateHosts, candNum)
	}

	return chooseHosts(hosts, clusterName, traceId)
}

var (
	inferLbCandidatePercent     = 5
	inferLbCandidatePercentOnce sync.Once
)

func getCandidatePercent() int {
	inferLbCandidatePercentOnce.Do(func() {
		env := os.Getenv("HTNN_AIGW_INFER_LB_CANDIDATE_PERCENT")
		if env != "" {
			if d, err := strconv.Atoi(env); err == nil {
				inferLbCandidatePercent = d
			}
		}
	})
	return inferLbCandidatePercent
}

func setLogField(ctx context.Context, k string, v interface{}) {
	callbacks, ok := ctx.Value(KeyFilterCallback).(filtermanager.FilterCallbackHandler)
	if !ok {
		api.LogErrorf("failed to get filter callback handler")
	} else {
		request.SetLogField(callbacks, k, v)
	}
}

func candidateNumFromContext(ctx context.Context, hosts []types.Host) int {
	percent := pkgcommon.GetValueFromCtx(ctx, KeyCandidatePercent, getCandidatePercent())
	api.LogDebugf("percent: %d, hosts: %d", percent, len(hosts))
	// at least 1, at most all
	return min(max(1, len(hosts)*percent/100), len(hosts))
}

type HostMatchInfo struct {
	CacheRatio float64 `json:"cache_ratio"`
}

func setHostMatchInfo(ctx context.Context, stat *EndpointStatsWrapper) context.Context {
	if stat.CacheStats != nil {
		hostInfo := &HostMatchInfo{
			CacheRatio: stat.CacheStats.CacheHitScore,
		}
		ctx = context.WithValue(ctx, KeyHostMatchInfo, hostInfo)
	}
	return ctx
}

// TODO: weighted by score
func selectHosts(hosts []types.Host) (int, types.Host) {
	// random for now
	i := rand.Intn(len(hosts))
	addr := hosts[i]
	return i, addr
}

func filterHostsBySelector(hosts []types.Host, selector map[string]string) []types.Host {
	var matchedHosts []types.Host
	for _, host := range hosts {
		match := true
		labels := host.Labels()
		for k, v := range selector {
			if labels[k] != v {
				match = false
				break
			}
		}
		if match {
			matchedHosts = append(matchedHosts, host)
		}
	}
	return matchedHosts
}

func chooseHosts(candidateHosts []types.Host, clusterName, traceId string) types.Host {
	i, addr := selectHosts(candidateHosts)
	api.LogInfof("choose %d th address %+v for cluster [%s], traceID: %s", i, addr.Address(), clusterName, traceId)
	return candidateHosts[i]
}

// order by desc score
func CompareEndpointStatsWrapperWithCache(m *EndpointStatsWrapper, other *EndpointStatsWrapper) int {
	return cmp.Compare(other.Score, m.Score)
}

// isModelLoadAwareEnable check whether load-aware is enabled,
// 1. check from ctx value KeyLoadAwareEnable first, if exists, use it;
// 2. if not exists, use global env variable
func isModelLoadAwareEnable(ctx context.Context) bool {
	if v := ctx.Value(KeyLoadAwareEnable); v != nil {
		if enable, ok := v.(bool); ok {
			return enable
		}
	}
	api.LogDebugf("use global metacenter load aware: %v", metadata_center.IsMetaDataCenterEnable())
	return metadata_center.IsMetaDataCenterEnable()
}

// isModelCacheAwareEnable check whether cache-aware is enabled,
// 1. check from ctx value KeyCacheAwareEnable first, if exists, use it;
// 2. if not exists, use global env variable
func isModelCacheAwareEnable(ctx context.Context) bool {
	if v := ctx.Value(KeyCacheAwareEnable); v != nil {
		if enable, ok := v.(bool); ok {
			return enable
		}
	}
	api.LogDebugf("use global metacenter cache aware: %v", metadata_center.IsMetaDataCenterCacheEnable())
	return metadata_center.IsMetaDataCenterCacheEnable()
}

func getClusterMetric(ctx context.Context, cluster string) (map[string]*mctypes.EndpointStats, error) {
	setLogField(ctx, KeyUseMetaLoad, 0)
	if isModelLoadAwareEnable(ctx) {
		metadataCenter := metadata_center.GetMetadataCenter()
		modelName := pkgcommon.GetValueFromCtx(ctx, KeyModelName, "")
		backend := pkgcommon.GetValueFromCtx(ctx, KeyBackendName, "")
		clusterName := pkgcommon.GetValueFromCtx(ctx, KeyClusterName, "")
		if clusterName != "" {
			start := time.Now()
			stats, err := metadataCenter.QueryLoad(ctx, clusterName)
			if err != nil {
				api.LogWarnf("get load metrics form metacenter error, fallback to use random. model name: %s, backend: %s, err: %+v", modelName, backend, err)
				return nil, fmt.Errorf("get load metrics form metacenter error, fallback to use random. model name: %s, backend: %s, err: %+v", modelName, backend, err)
			} else {
				api.LogDebugf("get load metrics from metacenter, model name: %s, backend: %s, stats: %v", modelName, backend, stats)
				setLogField(ctx, KeyUseMetaLoad, 1)
				duration := time.Since(start)
				if duration > 10*time.Millisecond {
					api.LogWarnf("get load metric from metacenter duration: %dms, modelname=%s, backend=%s", duration.Milliseconds(), modelName, backend)
				}
				setLogField(ctx, KeyLoadDuration, fmt.Sprintf("%.3fms", float64(duration.Microseconds())/1000.0))
				return stats, nil
			}
		}
	}

	return nil, fmt.Errorf("fallback to use random. cluster: %s", cluster)
}

var (
	// EndpointStats in EndpointStatsWrapper: won't be empty
	getEndpointStatsByClusterName = func(ctx context.Context, clusterName string, hosts []types.Host) ([]*EndpointStatsWrapper, error) {
		epStatsWrapper := make([]*EndpointStatsWrapper, len(hosts))
		epStats, err := getClusterMetric(ctx, clusterName)
		if err != nil {
			return nil, err
		}
		for i, host := range hosts {
			stat, ok := epStats[host.Ip()]
			if !ok {
				api.LogWarnf("endpoint stats not found for %s, clusterName %s", host.Ip(), clusterName)
				epStatsWrapper[i] = &EndpointStatsWrapper{
					Host: host,

					// Notice: bad upstream may easier to be lower load
					EndpointStats: &mctypes.EndpointStats{
						TotalReqs:    0,
						PromptLength: 0,
						PrefillReqs:  0,
					},
				}
			} else {
				epStatsWrapper[i] = &EndpointStatsWrapper{
					EndpointStats: stat,
					Host:          host,
				}
			}
		}

		return epStatsWrapper, nil
	}

	getEndpointCacheStats = func(ctx context.Context) (map[string]*EndpointCacheStats, error) {
		if isModelCacheAwareEnable(ctx) {
			metadataCenter := metadata_center.GetMetadataCenter()
			promptHash := pkgcommon.GetValueFromCtx(ctx, KeyPromptHash, []uint64{})
			modelName := pkgcommon.GetValueFromCtx(ctx, KeyModelName, "")
			clusterName := pkgcommon.GetValueFromCtx(ctx, KeyClusterName, "")
			if len(promptHash) == 0 || clusterName == "" {
				api.LogErrorf("get cluster cache form metdata center error,model name=%s, clusterName=%s.", modelName, clusterName)
				return nil, nil
			}

			start := time.Now()
			result, err := metadataCenter.QueryKVCache(ctx, clusterName, promptHash, metadata_center.DefaultTopK)

			// log cache query duration
			duration := time.Since(start)
			if duration > 10*time.Millisecond {
				api.LogWarnf("get cache from metacenter duration: %dms, modelname=%s, clusterName=%s", duration.Milliseconds(), modelName, clusterName)
			}
			setLogField(ctx, KeyCacheDuration, fmt.Sprintf("%.3fms", float64(duration.Microseconds())/1000.0))

			if err != nil {
				return nil, err
			}

			res := make(map[string]*EndpointCacheStats, len(result))
			for _, r := range result {
				res[r.Ip] = &EndpointCacheStats{
					CacheHitScore:  float64(r.Length) / float64(len(promptHash)),
					CacheHitLength: r.Length,
					NodeIP:         r.Ip,
				}
			}

			return res, err
		}

		return nil, nil // cache not enable, return nil and retied to use load metric
	}
)

func mergeEndpointsStatsWrapperCacheStats(ctx context.Context, load []*EndpointStatsWrapper, cache map[string]*EndpointCacheStats) []*EndpointStatsWrapper {
	var maxQueueSize float64 = 0
	var minQueueSize float64 = math.MaxFloat64
	// min prompt length is 1024, prefill time is small when less than 1024, reduce prefill weight
	maxPromptLength := 1024

	for _, stat := range load {
		if stat.EndpointStats != nil {
			size := stat.EndpointStats.TotalReqs
			maxQueueSize = math.Max(maxQueueSize, float64(size))
			minQueueSize = math.Min(minQueueSize, float64(size))

			if stat.EndpointStats.PromptLength > maxPromptLength {
				maxPromptLength = stat.EndpointStats.PromptLength
			}
		}
	}
	if minQueueSize == math.MaxFloat64 {
		minQueueSize = 0
	}

	cacheHitWeight := float64(pkgcommon.GetValueFromCtx(ctx, KeyCacheRatioWeight, InferLbCacheRatioWeight))

	// min range is 2, when the concurrency difference is small, cache hit rate weight is higher
	delta := math.Max(2, float64(maxQueueSize-minQueueSize))
	configRequestLoadWeight := float64(pkgcommon.GetValueFromCtx(ctx, KeyLoadRequestWeight, InferLbRequestLoadWeight))
	// request weight increased when delta is larger than 5
	requestLoadWeight := configRequestLoadWeight * math.Ceil(delta/5)

	prefillRadioWeight := float64(pkgcommon.GetValueFromCtx(ctx, KeyLoadPrefillWeight, InferLbPrefillLoadWeight))

	api.LogDebugf("cacheHitWeight: %f, requestLoadWeight: %f, prefillRadioWeight: %f", cacheHitWeight, requestLoadWeight, prefillRadioWeight)

	res := make([]*EndpointStatsWrapper, len(load))
	for i, stat := range load {
		stat.CacheHitRate = 0
		if cache != nil {
			if cacheStat, ok := cache[stat.Host.Ip()]; ok {
				stat.CacheStats = cacheStat
				stat.CacheHitRate = cacheStat.CacheHitScore
			}
		}

		stat.RequestLoad = 1.0
		stat.PrefillLoad = 0.0

		// stat.EndpointStats won't be empty, just for in case
		if stat.EndpointStats != nil {
			size := float64(stat.EndpointStats.TotalReqs) - minQueueSize
			stat.RequestLoad = size / float64(delta)
			stat.PrefillLoad = float64(stat.EndpointStats.PromptLength) / float64(maxPromptLength)
		}

		// score = W1 * cache_ratio - W2 * request_load - W3 * prefill_load
		stat.Score = cacheHitWeight*stat.CacheHitRate - requestLoadWeight*stat.RequestLoad - prefillRadioWeight*stat.PrefillLoad
		res[i] = stat
	}
	return res
}

// GetCandidateByStats get sorted hosts by metrics.
func (lb *inferenceLoadBalancer) GetCandidateByStats(ctx context.Context, clusterName string, hosts []types.Host, candNum int) []types.Host {
	// EndpointStats in EndpointStatsWrapper: won't be empty
	stats, err := getEndpointStatsByClusterName(ctx, clusterName, hosts)
	if err != nil {
		api.LogErrorf("failed to get endpoint stats by cluster name:%s, err: %+v", clusterName, err)
		return hosts
	}

	caches, err := getEndpointCacheStats(ctx)
	if err != nil { // failed means no cache
		api.LogInfof("failed to get cache stats by cluster name: %s, cache: %v, err: %+v", clusterName, caches, err)
	}

	useCache := 0
	if len(caches) > 0 {
		useCache = 1
	}
	setLogField(ctx, KeyUseMetaCache, useCache)
	stats = mergeEndpointsStatsWrapperCacheStats(ctx, stats, caches)
	slices.SortFunc(stats, CompareEndpointStatsWrapperWithCache)

	if api.GetLogLevel() <= api.Info {
		traceId := pkgcommon.GetValueFromCtx(ctx, KeyTraceId, "")
		for i, stat := range stats {
			// use for debugging
			if i < candNum+5 {
				api.LogInfof("the %d candidate for cluster [%s] are %s, traceID: %s", i, clusterName, stat, traceId)
			} else {
				api.LogDebugf("the %d candidate for cluster [%s] are %s, traceID: %s", i, clusterName, stat, traceId)
			}
		}
	}

	res := make([]types.Host, 0, len(hosts))
	for i := 0; i < candNum; i++ {
		ctx = setHostMatchInfo(ctx, stats[i])
		res = append(res, stats[i].Host)
	}

	return res
}
