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
	"hash"

	"github.com/twmb/murmur3"
)

const (
	DefaultTextChunkLen = 512
)

type HashConfig struct {
	ChunkLen int `json:"chunk_len"`
}

type Hash struct {
	config  *HashConfig
	newHash func() hash.Hash64
}

func NewHash(config *HashConfig) *Hash {
	return &Hash{
		config:  config,
		newHash: murmur3.New64,
	}
}

func (h *Hash) PromptToHash(prompt []byte) []uint64 {
	plen := len(prompt)
	if plen == 0 {
		return []uint64{}
	}

	numChunks := (plen + h.config.ChunkLen - 1) / h.config.ChunkLen

	buf := make([]uint64, 0, numChunks)
	hash := h.newHash()

	if plen <= h.config.ChunkLen {
		hash.Write(prompt)
		buf = append(buf, hash.Sum64())
		return buf
	}
	for start := 0; start < plen; start += h.config.ChunkLen {
		end := start + h.config.ChunkLen
		if end > plen {
			end = plen
		}
		chunk := prompt[start:end]
		hash.Write(chunk)
		buf = append(buf, hash.Sum64())
	}

	return buf
}
