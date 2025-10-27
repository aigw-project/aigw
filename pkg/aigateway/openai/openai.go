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

package openai

import (
	openaigo "github.com/openai/openai-go"
)

type Prompt struct {
	Messages []openaigo.ChatCompletionMessageParamUnion `json:"messages,omitempty"`
}

type CallTool struct {
	Id       string       `json:"id"`
	Type     string       `json:"type"`
	Function CallFunction `json:"function"`
}

type CallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAIChatCompletion openai protocol non-streaming response bdoy
type OpenAIChatCompletion struct {
	Id      string                   `json:"id"`
	Choices []ChatCompletionChoice   `json:"choices"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Object  string                   `json:"object"`
	Usage   openaigo.CompletionUsage `json:"usage"`
}

type ChatCompletionChoice struct {
	FinishReason string                 `json:"finish_reason"`
	Index        int                    `json:"index"`
	Message      ChatCompletionMessage  `json:"message"`
	Logprobs     ChatCompletionLogProbs `json:"logprobs,omitempty"`
}

type ChatCompletionMessage struct {
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Role             string     `json:"role"`
	ToolCalls        []CallTool `json:"tool_calls,omitempty"`
	Refusal          string     `json:"refusal,omitempty"`
}

type ChatCompletionLogProbs struct {
	Content []ChatCompletionLogProbsContent `json:"content"`
}

type ChatCompletionLogProbsContent struct {
	Token    string  `json:"token"`
	Lobprobs float64 `json:"lobprobs"`
	Bytes    []int   `json:"bytes"`
}

// OpenAIChatCompletionChunk OpenAi protocol streaming response body
type OpenAIChatCompletionChunk struct {
	Id      string                      `json:"id"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Object  string                      `json:"object"`
	Usage   openaigo.CompletionUsage    `json:"usage,omitempty"`
}

type ChatCompletionChunkChoice struct {
	FinishReason string                   `json:"finish_reason,omitempty"`
	Index        int                      `json:"index"`
	Delta        ChatCompletionChunkDelta `json:"delta"`
	Logprobs     ChatCompletionLogProbs   `json:"logprobs,omitempty"`
}

type ChatCompletionChunkDelta struct {
	Content          *string    `json:"content"`
	ReasoningContent *string    `json:"reasoning_content"`
	Role             string     `json:"role,omitempty"`
	ToolCalls        []CallTool `json:"tool_calls,omitempty"`
	Refusal          string     `json:"refusal,omitempty"`
}
