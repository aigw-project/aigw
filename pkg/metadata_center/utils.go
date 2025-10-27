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
	"hash/fnv"
	"os"
	"strconv"
	"sync"

	"mosn.io/htnn/api/pkg/filtermanager/api"

	"github.com/aigw-project/aigw/pkg/metadata_center/servicediscovery"
)

const (
	AigwMetaDataCenterEnable = "AIGW_META_DATA_CENTER_ENABLE"

	AigwMetaDataCenterCacheEnable = "AIGW_META_DATA_CENTER_CACHE_ENABLE"
)

var (
	// metaDataCenterEnable default value: true
	metaDataCenterEnable     = true
	metaDataCenterEnableOnce sync.Once

	metaDataCenterHost string

	// metaDataCenterCacheEnable default value: false
	metaDataCenterCacheEnable = false
)

func hashKey(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// IsMetaDataCenterEnable
func IsMetaDataCenterEnable() bool {
	metaDataCenterEnableOnce.Do(func() {
		env := os.Getenv(AigwMetaDataCenterEnable)
		if env != "" {
			api.LogDebugf("metadata center enable:%v", env)
			if d, err := strconv.ParseBool(env); err == nil {
				metaDataCenterEnable = d
			} else {
				api.LogErrorf("metadata center enable parse error:%v", err)
				return
			}
		}

		if !metaDataCenterEnable {
			api.LogErrorf("metadata center enable is false")
			return
		}

		// worked in discovery
		env = os.Getenv(servicediscovery.AigwMetaDataCenter_Host)
		if env != "" {
			api.LogDebugf("metadata center host:%v", env)
			metaDataCenterHost = env
		} else { // host is empty, disable metadata center
			api.LogErrorf("metadata center host is empty")
			metaDataCenterEnable = false
			return
		}

		env = os.Getenv(AigwMetaDataCenterCacheEnable)
		if env != "" { // cache enable is empty, use default value: false
			api.LogDebugf("metadata center cache enable:%v", env)
			if d, err := strconv.ParseBool(env); err == nil {
				metaDataCenterCacheEnable = d
			}
		}
	})
	return metaDataCenterEnable
}

func IsMetaDataCenterCacheEnable() bool {
	return metaDataCenterCacheEnable
}
