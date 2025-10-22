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

package llmproxy

import (
	"context"
	"time"

	"github.com/google/uuid"
	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/aigateway/loadbalancer/inferencelb"
	mctypes "github.com/aigw-project/aigw/pkg/metadata_center/types"
	"github.com/aigw-project/aigw/pkg/metrics_stats"
	"github.com/aigw-project/aigw/pkg/request"
	"github.com/aigw-project/aigw/plugins/llmproxy/transcoder"
)

func (f *filter) UniqueId() string {
	if f.uniqueId == "" {
		f.uniqueId = uuid.New().String()
		request.SetLogField(f.callbacks, "metacenter_reqid", f.uniqueId)
	}
	return f.uniqueId
}

func (f *filter) AddRequest() {
	if !f.isModelLoadAwareEnable() {
		api.LogDebugf("model load aware is not enable, model name: %s", f.modelName)
		return
	}

	ctx := context.WithValue(context.Background(), mctypes.CtxKeyTraceID, f.traceId)
	err := f.config.MC.AddRequest(ctx, f.UniqueId(), f.cluster, f.serverIp, f.promptLength)
	if err != nil {
		api.LogErrorf("increase model stats failed, traceid: %v, err: %v", f.traceId, err)
		return
	}

	api.LogDebugf("increase model stats success, model name: %s, backend: %s, ip: %s, prompt length=%d", f.modelName, f.backendProtocol, f.serverIp, f.promptLength)
	f.isIncreaseRecorded = true

	if !f.isStream {
		ttft := metrics_stats.MatchTTFT(f.modelName, f.promptLength)
		ms := time.Duration(ttft*12/10) * time.Millisecond

		api.LogInfof("non-stream request, start prompt decrease timer, model name: %s, trace id=%s, predict ttft=%dms", f.modelName, f.traceId, ttft)
		f.promptDecreaseTimer = time.AfterFunc(ms, func() {
			f.DeletePromptLength()
		})
	}
}

func (f *filter) DecreaseMetaDataCenter() {
	if !f.isModelLoadAwareEnable() {
		api.LogDebugf("metadata center load aware is not enable, model name: %s", f.modelName)
		return
	}

	f.StopPromptDecreaseTimer()

	ctx := context.WithValue(context.Background(), mctypes.CtxKeyTraceID, f.traceId)
	err := f.config.MC.DeleteRequest(ctx, f.UniqueId())
	if err != nil {
		api.LogErrorf("decrease model stats failed, traceid: %s, err: %v", f.traceId, err)
	}
	api.LogDebugf("decrease model stats success, model name: %s, backend: %s, ip: %s", f.modelName, f.backendProtocol, f.serverIp)
}

func (f *filter) StopPromptDecreaseTimer() {
	timer := f.promptDecreaseTimer
	if timer != nil {
		api.LogDebugf("stopping prompt decrease timer, model name: %s", f.modelName)
		f.promptDecreaseTimer = nil

		// it's safe to invoke Stop() again even if the timer has already expired
		timer.Stop()
	}
}

func (f *filter) DeletePromptLength() {
	if !f.isModelLoadAwareEnable() {
		api.LogDebugf("metadata center load aware is not enable, model name: %s", f.modelName)
		return
	}
	if !f.isIncreaseRecorded {
		api.LogDebugf("prompt length is not increased, no need to decrease, model name: %s，trace id=%s", f.modelName, f.traceId)
		return
	}
	if f.isPromptLengthDeleted {
		api.LogWarnf("prompt length is already deleted, model name: %s，trace id=%s", f.modelName, f.traceId)
		return
	}

	f.StopPromptDecreaseTimer()

	ctx := context.WithValue(context.Background(), mctypes.CtxKeyTraceID, f.traceId)
	err := f.config.MC.DeleteRequestPrompt(ctx, f.UniqueId())
	if err != nil {
		api.LogErrorf("decrease prompt length failed, traceid: %s, err: %v", f.traceId, err)
	}
	f.isPromptLengthDeleted = true
	api.LogDebugf("decrease prompt length, model name: %s, backend: %s, ip: %s, prompt length: %d, trace id: %s", f.modelName, f.backendProtocol, f.serverIp, f.promptLength, f.traceId)
}

func (f *filter) SaveKVCache(header api.ResponseHeaderMap) {
	// TODO
}

func (f *filter) setPromptsContext(ctx context.Context) context.Context {
	if f.isModelCacheAwareEnable() {
		ctx = context.WithValue(ctx, inferencelb.KeyPromptHash, f.promptHash)
		return ctx
	}
	return ctx
}

func (f *filter) setLoadBalanceConfig(ctx context.Context, modelName string) context.Context {
	ruleConfig := f.config.FindLbMappingRule(modelName)
	if ruleConfig != nil {
		api.LogDebugf("set load balance config to context, model name: %s, config: %v", modelName, ruleConfig)
		ctx = context.WithValue(ctx, inferencelb.KeyLoadAwareEnable, ruleConfig.LoadAwareEnable)
		ctx = context.WithValue(ctx, inferencelb.KeyCacheAwareEnable, ruleConfig.CacheAwareEnable)
		// must be an int type, to align with GetValueFromCtx with int default value
		ctx = context.WithValue(ctx, inferencelb.KeyCandidatePercent, int(ruleConfig.CandidatePercent))

		api.LogDebugf("set load balance config to context, model name: %s, new version", modelName)
		ctx = context.WithValue(ctx, inferencelb.KeyLoadRequestWeight, int(ruleConfig.RequestLoadWeight))
		ctx = context.WithValue(ctx, inferencelb.KeyLoadPrefillWeight, int(ruleConfig.PrefillLoadWeight))
		ctx = context.WithValue(ctx, inferencelb.KeyCacheRatioWeight, int(ruleConfig.CacheRadioWeight))
	}
	return ctx
}

func (f *filter) isModelLoadAwareEnable() bool {
	if f.config.LbMappingConfigs == nil {
		return true // default enable
	}
	if lbConfig := f.config.FindLbMappingRule(f.modelName); lbConfig != nil {
		return lbConfig.LoadAwareEnable
	}
	return true
}

func (f *filter) isModelCacheAwareEnable() bool {
	if f.config.LbMappingConfigs == nil {
		return false
	}
	if lbConfig := f.config.FindLbMappingRule(f.modelName); lbConfig != nil {
		return lbConfig.CacheAwareEnable
	}
	return true
}

func (f *filter) PromptDataHash(promptContext *transcoder.PromptMessageContext) {
	if promptContext == nil {
		api.LogErrorf("get prompt hash failed, promptContext is nil")
		return
	}
	f.promptLength = len(promptContext.PromptContent)

	if !f.isModelCacheAwareEnable() {
		api.LogDebugf("model cache aware is not enable, model name: %s", f.modelName)
		return
	}
	if len(promptContext.PromptContent) > 0 {
		h := NewHash(&HashConfig{
			ChunkLen: DefaultTextChunkLen,
		})
		f.promptHash = h.PromptToHash(promptContext.PromptContent)
	}
	if promptContext.IsVlModel {
		request.SetLogField(f.callbacks, "is_vl", 1)
	} else {
		request.SetLogField(f.callbacks, "is_vl", 0)
	}
	api.LogDebugf("prompt hash: %v, prompt length: %d, trace_id=%s", f.promptHash, f.promptLength, f.traceId)
}
