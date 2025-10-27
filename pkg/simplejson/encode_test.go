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

package simplejson

import (
	"testing"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
	"github.com/stretchr/testify/assert"
	"mosn.io/htnn/api/plugins/tests/pkg/envoy"
)

type A struct {
	B *B
}

type B struct {
	A *A
}

func TestJsonEncode(t *testing.T) {
	cb := envoy.NewFilterCallbackHandler()
	api.SetCommonCAPI(cb)

	a := &A{}
	b := &B{}
	a.B = b
	b.A = a

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name: "sanity",
			input: map[string]any{
				"key1": "value1",
			},
			want: `{"key1":"value1"}`,
		},
		{
			name:  "failed",
			input: a,
			want:  ``,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := string(Encode(test.input))
			assert.Equal(t, test.want, got)
		})
	}
}
