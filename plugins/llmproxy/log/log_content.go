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

package log

import (
	"bytes"
	"encoding/json"

	openaigo "github.com/openai/openai-go"

	"github.com/aigw-project/aigw/pkg/aigateway/openai"
)

type TokenUsage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
}

type LLMLogItems struct {
	RawRequest      []byte
	RawResponse     *bytes.Buffer
	ThinkingContent *bytes.Buffer
	ErrorMessage    string
	usage           TokenUsage

	Env       string
	ModelName string
}

type LLMLogOperation interface {
	GetRequest() string
	SetRequest(req []byte)

	GetResponse() string
	GetThinkingResponse() string

	AppendOpenAIResponse(res *openaigo.Completion)
	AppendManualOpenAIChunkResponse(res *openai.OpenAIChatCompletionChunk)
	AppendManualOpenAIResponse(res *openai.OpenAIChatCompletion)
	SetErrorMessage(msg string)

	GetUsage() TokenUsage

	GetEnv() string
	GetModelName() string
}

func (logItems *LLMLogItems) GetRequest() string {
	return string(logItems.RawRequest)
}

func (logItems *LLMLogItems) SetRequest(req []byte) {
	var compactReq bytes.Buffer
	err := json.Compact(&compactReq, req)
	if err != nil {
		logItems.RawRequest = req
	} else {
		logItems.RawRequest = compactReq.Bytes()
	}
}

func (logItems *LLMLogItems) GetResponse() string {
	if logItems.RawResponse == nil {
		return ""
	}
	return logItems.RawResponse.String()
}

func (logItems *LLMLogItems) GetThinkingResponse() string {
	if logItems.ThinkingContent == nil {
		return ""
	}
	return logItems.ThinkingContent.String()
}

func (logItems *LLMLogItems) AppendOpenAIResponse(res *openaigo.Completion) {
	if res == nil {
		return
	}
	for _, choice := range res.Choices {
		buffer := []byte(choice.Text)
		if logItems.RawResponse == nil {
			logItems.RawResponse = bytes.NewBuffer(buffer)
		} else {
			logItems.RawResponse.Write(buffer)
		}
	}

	logItems.appendUsage(res.Usage)
}

func (logItems *LLMLogItems) AppendManualOpenAIChunkResponse(chunk *openai.OpenAIChatCompletionChunk) {
	if chunk == nil {
		return
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != nil {
			content := []byte(*choice.Delta.Content)
			trimContent := content
			if logItems.RawResponse == nil {
				logItems.RawResponse = bytes.NewBuffer(trimContent)
			} else {
				logItems.RawResponse.Write(trimContent)
			}
		}

		if choice.Delta.ReasoningContent != nil {
			reasoning := []byte(*choice.Delta.ReasoningContent)
			trimReasoningContent := reasoning
			if logItems.ThinkingContent == nil {
				logItems.ThinkingContent = bytes.NewBuffer(trimReasoningContent)
			} else {
				logItems.ThinkingContent.Write(trimReasoningContent)
			}
		}
	}

	logItems.appendUsage(chunk.Usage)
}

func (logItems *LLMLogItems) SetErrorMessage(msg string) {
	logItems.ErrorMessage = msg
}

func (logItems *LLMLogItems) GetErrorMessage() string {
	return logItems.ErrorMessage
}

func (logItems *LLMLogItems) AppendManualOpenAIResponse(res *openai.OpenAIChatCompletion) {
	if res == nil {
		return
	}
	for _, choice := range res.Choices {
		content := []byte(choice.Message.Content)
		trimContent := content
		if logItems.RawResponse == nil {
			logItems.RawResponse = bytes.NewBuffer(trimContent)
		} else {
			logItems.RawResponse.Write(trimContent)
		}

		reasoning := []byte(choice.Message.ReasoningContent)
		trimReasoningContent := reasoning
		if logItems.ThinkingContent == nil {
			logItems.ThinkingContent = bytes.NewBuffer(trimReasoningContent)
		} else {
			logItems.ThinkingContent.Write(trimReasoningContent)
		}
	}

	logItems.appendUsage(res.Usage)
}

func (logItems *LLMLogItems) appendUsage(usage openaigo.CompletionUsage) {
	if usage.TotalTokens > logItems.usage.TotalTokens {
		logItems.usage.PromptTokens = usage.PromptTokens
		logItems.usage.CompletionTokens = usage.CompletionTokens
		logItems.usage.TotalTokens = usage.TotalTokens
		if usage.PromptTokensDetails.CachedTokens != 0 {
			logItems.usage.CachedTokens = usage.PromptTokensDetails.CachedTokens
		}
	}
}

func (logItems *LLMLogItems) GetUsage() TokenUsage {
	return logItems.usage
}

func (logItems *LLMLogItems) GetEnv() string {
	return logItems.Env
}

func (logItems *LLMLogItems) GetModelName() string {
	return logItems.ModelName
}
