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

package metadata_center

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"

	"github.com/aigw-project/aigw/pkg/async_request"
	pkgcommon "github.com/aigw-project/aigw/pkg/common"
	"github.com/aigw-project/aigw/pkg/metadata_center/servicediscovery"
	"github.com/aigw-project/aigw/pkg/metadata_center/types"
	"github.com/aigw-project/aigw/pkg/prom"
)

const (
	DefaultTopK     = 10
	DefaultChunkLen = 512

	TraceIdHeader = "TraceId"

	MetaDataCenterLoadPath         = "/v1/load/stats"
	MetaDataCenterLoadPromptLength = "/v1/load/prompt"
	MetaDataCenterCacheFetchPath   = "/v1/cache/query"
	MetaDataCenterCacheSavePath    = "/v1/cache/save"

	AigwMetaDataCenter_MaxFailoverRetry = "AIGW_META_MAX_FAILOVER_RETRY"
	AigwMetaDataCenter_WorkerCount      = "AIGW_METADATA_CENTER_WORKER_COUNT"
	AigwMetaDataCenter_MaxRetry         = "AIGW_METADATA_CENTER_MAX_RETRY"
	AigwMetaDataCenter_QueueSize        = "AIGW_METADATA_CENTER_QUEUE_SIZE"

	AigwMetaDataCenter_UpdateStatsTimeout = "AIGW_METADATA_CENTER_UPDATE_STATS_TIMEOUT"
	AigwMetaDataCenter_FetchMetricTimeout = "AIGW_METADATA_CENTER_FETCH_METRIC_TIMEOUT"
	AigwMetaDataCenter_FetchCacheTimeout  = "AIGW_META_DATA_CACHE_FETCH_TIMEOUT"

	AigwMetaDataCenterClient_Timeout            = "AIGW_METADATA_CENTER_CLIENT_TIMEOUT"
	AigwMetaDataCenterClient_IdelConnectTimeout = "AIGW_METADATA_CENTER_CLIENT_IDEL_CONNECT_TIMEOUT"
	AigwMetaDataCenterClient_MaxIdleConns       = "AIGW_METADATA_CENTER_CLIENT_MAX_IDLE_CONNS"
	AigwMetaDataCenterClient_KeepAlive          = "AIGW_METADATA_CENTER_CLIENT_KEEPALIVE"
)

const (
	MetaCenterTraceId pkgcommon.LBCtxKey = "metacenter.traceId"
)

var (
	metaDataCenter *MetaDataCenter

	metaDataCenterFetchMetricTimeout     = 100 //ms
	metaDataCenterFetchMetricTimeoutOnce sync.Once

	metaDataCenterFetchCacheTimeout     = 100 //ms
	metaDataCenterFetchCacheTimeoutOnce sync.Once
)

type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

type MetaCenterResponse struct {
	Status  string     `json:"status"`
	Error   *ErrorInfo `json:"error"`
	TraceID string     `json:"trace_id,omitempty"`
}

type InferenceRequest struct {
	RequestId    string `json:"request_id" binding:"required"`
	Cluster      string `json:"cluster" binding:"required"`
	PromptLength int    `json:"prompt_length,omitempty"`
	Ip           string `json:"ip" binding:"required"`
	TimeStamp    int64  `json:"timestamp,omitempty"`
	// trace id from request header
	TraceId string `json:"-"`
}

type EngineStatsJSON struct {
	Ip           string `json:"ip"`
	QueuedReqNum int32  `json:"queued_req_num"`
	PromptLength int32  `json:"prompt_length"`
	UpdatedTime  int64  `json:"updated_time"`
}

type CacheQueryParam struct {
	Cluster    string   `json:"cluster" binding:"required"`
	PromptHash []uint64 `json:"prompt_hash" binding:"required"`
	TopK       int      `json:"topk"`
}

type LocationResponse struct {
	Ip     string `json:"ip"`
	Length int    `json:"length"`
}

type CacheQueryResponse struct {
	Locations []*LocationResponse `json:"locations"`
}

type CacheSaveParam struct {
	Cluster    string   `json:"cluster" binding:"required"`
	PromptHash []uint64 `json:"prompt_hash" binding:"required"`
	Ip         string   `json:"ip" binding:"required"`
}

type RequestParam struct {
	TraceId string
	HashKey string
	Method  string
	Path    string
	Query   map[string]string
	Body    []byte
	Timeout time.Duration
}

func getPromptLength(prompt []byte) int {
	l := len(prompt)
	if l == 0 {
		return 0
	}
	// 计算分块数量
	return (l + DefaultChunkLen - 1) / DefaultChunkLen
}

type MetaDataCenter struct {
	asyncQueue       *async_request.AsyncQueue
	dateCenterClient *MetaDataCenterClient
}

func GetMetaDataCenterFetchMetricTimeout() int {
	metaDataCenterFetchMetricTimeoutOnce.Do(func() {
		metaDataCenterFetchMetricTimeout = pkgcommon.GetIntFromEnv(AigwMetaDataCenter_FetchMetricTimeout, 100)
	})
	return metaDataCenterFetchMetricTimeout
}

func GetMetaDataCenterFetchCacheTimeout() int {
	metaDataCenterFetchCacheTimeoutOnce.Do(func() {
		metaDataCenterFetchCacheTimeout = pkgcommon.GetIntFromEnv(AigwMetaDataCenter_FetchCacheTimeout, 100)
	})
	return metaDataCenterFetchCacheTimeout
}

// GetMetaDataCenterInstance always return the instance of MetaDataCenter
func NewMetaCenter() types.MetadataCenter {
	client := NewMetaDataCenterClient()
	timeout := pkgcommon.GetIntFromEnv(AigwMetaDataCenter_UpdateStatsTimeout, 100)
	cfg := async_request.Config{
		QueueSize:      pkgcommon.GetIntFromEnv(AigwMetaDataCenter_QueueSize, 1000),
		WorkerCount:    pkgcommon.GetIntFromEnv(AigwMetaDataCenter_WorkerCount, 100),
		MaxRetries:     pkgcommon.GetIntFromEnv(AigwMetaDataCenter_MaxRetry, 0),
		DefaultTimeout: time.Duration(timeout) * time.Millisecond,
	}
	asyncQueue := async_request.NewAsyncQueue(cfg, client)
	metaDataCenter = &MetaDataCenter{
		asyncQueue:       asyncQueue,
		dateCenterClient: client,
	}
	api.LogInfof("metadata center init success, config:%v", cfg)

	return metaDataCenter
}

func (mc *MetaDataCenter) AddRequest(ctx context.Context, requestId, cluster, ip string, promptLength int) error {
	traceId := pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, "")
	req := &InferenceRequest{
		RequestId:    requestId,
		Cluster:      cluster,
		Ip:           ip,
		PromptLength: promptLength,
		TimeStamp:    time.Now().UnixNano(),
		TraceId:      traceId,
	}

	body, err := json.Marshal(req)
	if err != nil {
		api.LogErrorf("json marshal error, req:%v, err:%v", req, err)
		return err
	}
	var task = &async_request.Task{
		HashKey: req.Cluster,
		Method:  http.MethodPost,
		URL:     MetaDataCenterLoadPath,
		Body:    body,
		TraceId: req.TraceId,
	}

	if err := mc.asyncQueue.Dispatch(*task); err != nil {
		api.LogErrorf("increase model stats, req:%v, err:%v", req, err)
		return err
	}
	api.LogDebugf("increase model stats, req:%v", req)
	return nil
}

func (mc *MetaDataCenter) DeleteRequest(ctx context.Context, requestId string) error {
	traceId := pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, "")
	req := &InferenceRequest{
		RequestId: requestId,
		TraceId:   traceId,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var task = &async_request.Task{
		HashKey: req.Cluster,
		Method:  http.MethodDelete,
		URL:     MetaDataCenterLoadPath,
		Body:    body,
		TraceId: req.TraceId,
	}

	if err := mc.asyncQueue.Dispatch(*task); err != nil {
		api.LogErrorf("decrease model stats error, req:%v, err:%v", req, err)
		return err
	}

	api.LogDebugf("decrease model stats success, req:%v", req)
	return nil
}

// updateMetacenterMetrics update metacenter metrics
func updateMetacenterMetrics(instance, method, path string, duration int64) {
	prom.MetacenterRequestsTotal.WithLabelValues(instance, method, path).Inc()
	prom.MetacenterRequestDuration.WithLabelValues(instance, method, path).Observe(float64(duration))
}

func (mc *MetaDataCenter) DeleteRequestPrompt(ctx context.Context, requestId string) error {
	traceId := pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, "")
	req := &InferenceRequest{
		RequestId: requestId,
		TraceId:   traceId,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var task = &async_request.Task{
		HashKey: req.Cluster,
		Method:  http.MethodDelete,
		URL:     MetaDataCenterLoadPromptLength,
		Body:    body,
		TraceId: req.TraceId,
	}

	if err := mc.asyncQueue.Dispatch(*task); err != nil {
		api.LogErrorf("decrease prompt length error, req:%v, err:%v", req, err)
		return err
	}

	api.LogDebugf("decrease prompt length success, req:%v", req)
	return nil
}

func (mc *MetaDataCenter) QueryLoad(ctx context.Context, cluster string) (map[string]*types.EndpointStats, error) {
	traceId := pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, "")
	body, err := mc.dateCenterClient.doRequestWithRetry(ctx, RequestParam{
		TraceId: traceId,
		HashKey: cluster,
		Method:  http.MethodGet,
		Path:    MetaDataCenterLoadPath,
		Query:   map[string]string{"cluster": cluster},
		Body:    nil,
		Timeout: time.Duration(GetMetaDataCenterFetchMetricTimeout()) * time.Millisecond,
	})

	if err != nil {
		api.LogErrorf("load request: read response body failed, err: %v", err)
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	type metricResponse struct {
		MetaCenterResponse
		ModelStats []EngineStatsJSON `json:"data"`
	}
	var response metricResponse
	if err := json.Unmarshal(body, &response); err != nil {
		api.LogErrorf("load request: parse metric response error: %v", err)
		return nil, err
	}

	revisePromptLength := func(len int, ip, trace string) int {
		if len < 0 {
			api.LogErrorf("load request[%s]: %s prompt length is negative, len: %d", trace, ip, len)
			return 0
		}
		return len
	}

	stats := make(map[string]*types.EndpointStats, len(response.ModelStats))
	for _, engine := range response.ModelStats {
		stats[engine.Ip] = &types.EndpointStats{
			PromptLength: revisePromptLength(int(engine.PromptLength), engine.Ip, traceId),
			PrefillReqs:  0, // TODO
			TotalReqs:    int(engine.QueuedReqNum),
		}
	}
	api.LogDebugf("metadata center response:%s", string(body))

	return stats, nil
}

type MetaDataCenterClient struct {
	client           *http.Client
	failoverRetry    int
	serviceDiscovery *servicediscovery.ServiceDiscovery
}

func NewMetaDataCenterClient() *MetaDataCenterClient {
	dialer := &net.Dialer{
		Timeout:   pkgcommon.GetDurationFromEnv(AigwMetaDataCenterClient_Timeout, 100*time.Millisecond),
		KeepAlive: pkgcommon.GetDurationFromEnv(AigwMetaDataCenterClient_KeepAlive, 10*time.Second),
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			MaxIdleConns:        pkgcommon.GetIntFromEnv(AigwMetaDataCenterClient_MaxIdleConns, 1024),
			MaxConnsPerHost:     pkgcommon.GetIntFromEnv(AigwMetaDataCenterClient_MaxIdleConns, 1024),
			MaxIdleConnsPerHost: pkgcommon.GetIntFromEnv(AigwMetaDataCenterClient_MaxIdleConns, 1024),
			IdleConnTimeout:     pkgcommon.GetDurationFromEnv(AigwMetaDataCenterClient_IdelConnectTimeout, 5*time.Minute),
		},
	}

	retryTimes := pkgcommon.GetIntFromEnv(AigwMetaDataCenter_MaxFailoverRetry, 1)

	return &MetaDataCenterClient{
		client:           client,
		failoverRetry:    retryTimes,
		serviceDiscovery: servicediscovery.GlobalServiceDiscovery,
	}
}

func (m *MetaDataCenterClient) doRequestWithRetry(ctx context.Context, reqParam RequestParam) ([]byte, error) {
	candidates := service.GetHosts(reqParam.HashKey, m.failoverRetry+1)
	if len(candidates) == 0 {
		api.LogErrorf("[TraceID: %s] no available host to send hash request method: %s, url: %s", reqParam.TraceId, reqParam.Method, reqParam.Path)
		return nil, errors.New("no available host")
	}

	var lastErr error
	start := time.Now()
	for attempt, host := range candidates {
		newContext, cancel := context.WithTimeout(ctx, reqParam.Timeout)
		respBody, err := m.doSingleRequest(newContext, host, reqParam)
		cancel()
		if err == nil {
			m.serviceDiscovery.ReportSuccess(host)
			updateMetacenterMetrics(host, reqParam.Method, reqParam.Path, time.Since(start).Microseconds())
			return respBody, nil
		}

		lastErr = err
		m.serviceDiscovery.ReportFailure(host)
		api.LogWarnf("[TraceID: %s] request attempt %d/%d to host %s failed, error: %v. Retrying...", reqParam.TraceId, attempt+1, len(candidates), host, err)
	}
	return nil, fmt.Errorf("all %d attempts failed for hosts %v, last error: %w", len(candidates), candidates, lastErr)
}

func (m *MetaDataCenterClient) HandleRequest(ctx context.Context, task async_request.Task) error {
	respBody, err := m.doRequestWithRetry(ctx, RequestParam{
		TraceId: task.TraceId,
		HashKey: task.HashKey,
		Method:  task.Method,
		Path:    task.URL,
		Body:    task.Body,
		Timeout: task.Timeout,
	})

	if err != nil {
		return fmt.Errorf("handel async request: metadata center request failed, respBody:%s, err:%v", string(respBody), err)
	}

	return nil
}

func (m *MetaDataCenterClient) doSingleRequest(ctx context.Context, host string, reqParam RequestParam) ([]byte, error) {
	bodyReader := bytes.NewReader(reqParam.Body)
	reqBody := io.NopCloser(bodyReader)
	port := service.GetPort()

	reqUrl := fmt.Sprintf("http://%s:%d%s", host, port, reqParam.Path)
	api.LogDebugf("[TraceID: %s] create request method:%s url: %s", reqParam.TraceId, reqParam.Method, reqUrl)
	req, err := http.NewRequestWithContext(ctx, reqParam.Method, reqUrl, reqBody)
	if err != nil {
		api.LogDebugf("[TraceID: %s] request: create request failed, err:%v", reqParam.TraceId, err)
		return nil, fmt.Errorf("request: create request failed, err:%w", err)
	}

	req.ContentLength = int64(bodyReader.Len())
	req.Header.Set(TraceIdHeader, reqParam.TraceId)

	if reqParam.Query != nil {
		for k, v := range reqParam.Query {
			queryParams := req.URL.Query()
			queryParams.Set(k, v)
			req.URL.RawQuery = queryParams.Encode()
		}
	}

	resp, err := m.client.Do(req)
	if err != nil {
		api.LogDebugf("[TraceID: %s] request: metadata center http request failed, err: %v", reqParam.TraceId, err)
		return nil, fmt.Errorf("request: metadata center http request failed, err: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("request: failed to read response body, err: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %v, body: %s", resp.StatusCode, string(b))
	}

	api.LogDebugf("[TraceID: %s]request: metadata center http has succeeded", reqParam.TraceId)
	return b, nil
}

func (mc *MetaDataCenter) QueryKVCache(ctx context.Context, cluster string, promptHash []uint64, topK int) ([]*types.KVCacheLocation, error) {
	param := &CacheQueryParam{
		Cluster:    cluster,
		PromptHash: promptHash,
		TopK:       topK,
	}

	reqBody, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("cache request: failed to marshal cache request body: %v", err)
	}

	body, err := mc.dateCenterClient.doRequestWithRetry(ctx, RequestParam{
		TraceId: pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, ""),
		HashKey: param.Cluster,
		Method:  http.MethodPost,
		Path:    MetaDataCenterCacheFetchPath,
		Query:   nil,
		Body:    reqBody,
		Timeout: time.Duration(GetMetaDataCenterFetchCacheTimeout()) * time.Millisecond,
	})

	if err != nil {
		api.LogErrorf("cache request: create request failed, err:%v", err)
		return nil, fmt.Errorf("failed to create cache request: %v", err)
	}

	type cacheQueryResponse struct {
		Data CacheQueryResponse `json:"data"`
		MetaCenterResponse
	}
	var response cacheQueryResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("cache request: parse cache response error: %s", err.Error())
	}
	if len(response.Data.Locations) == 0 {
		return nil, fmt.Errorf("cache request: cache response data is empty")
	}
	cacheResponse := response.Data

	stats := make([]*types.KVCacheLocation, len(response.Data.Locations))
	for _, item := range cacheResponse.Locations {
		stat := &types.KVCacheLocation{
			Ip:     item.Ip,
			Length: item.Length,
		}
		stats = append(stats, stat)
	}

	api.LogDebugf("metadata center cache stats:%v", stats)
	return stats, nil
}

func (mc *MetaDataCenter) SaveKVCache(ctx context.Context, cluster, ip string, promptHash []uint64) error {
	traceId := pkgcommon.GetValueFromCtx(ctx, MetaCenterTraceId, "")
	req := &CacheSaveParam{
		Cluster:    cluster,
		Ip:         ip,
		PromptHash: promptHash,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	var task = &async_request.Task{
		HashKey: req.Cluster,
		Method:  http.MethodPost,
		URL:     MetaDataCenterCacheSavePath,
		Body:    body,
		TraceId: traceId,
	}

	if err := mc.asyncQueue.Dispatch(*task); err != nil {
		api.LogErrorf("save cache error, req:%+v, err:%+v", req, err)
		return err
	}

	api.LogDebugf("save cache success, trace id:%s, cluster:%s, ip:%v, prompt_hash:[%v]", traceId, cluster, ip, req.PromptHash)
	return nil
}
