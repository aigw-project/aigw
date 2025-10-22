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
	"github.com/aigw-project/aigw/pkg/metadata_center/types"
)

var (
	instance types.MetadataCenter
	service  types.Service
)

func RegisterMetadataCenter(mc types.MetadataCenter) {
	instance = mc
}

func GetMetadataCenter() types.MetadataCenter {
	return instance
}

func RegsiterService(s types.Service) {
	service = s
}

func init() {
	// The default metadata center
	instance = NewMetaCenter()
}
