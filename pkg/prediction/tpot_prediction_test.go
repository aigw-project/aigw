// Copyright The AIGW Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prediction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTpotPredictorInit(t *testing.T) {
	p := &TpotPredictor{}
	p.Init([]uint64{10, 50, 100})

	if !assert.Equal(t, p.Params()[0], map[string]float64{"a": 0, "b": 0, "c": 0}) {
		t.Fatalf("RLS not initialized correctly")
	}
}

func TestSegment(t *testing.T) {
	p := &TpotPredictor{}
	p.Init([]uint64{10, 20, 30})

	assert.Equal(t, 0, p.segment(5))
	assert.Equal(t, 1, p.segment(10))
	assert.Equal(t, 1, p.segment(15))
	assert.Equal(t, 2, p.segment(25))
	assert.Equal(t, 3, p.segment(100))
}

func TestTrainAndPredict(t *testing.T) {
	p := &TpotPredictor{}
	p.Init([]uint64{10})

	var batchsize uint64 = 5
	var totalTokenNum uint64 = 100
	y := 50.0
	p.Train(batchsize, totalTokenNum, y)

	out := p.Predict(batchsize, totalTokenNum)
	if out == 0 {
		t.Fatalf("predict should produce non-zero after training; got %v", out)
	}
}

func TestParams(t *testing.T) {
	p := &TpotPredictor{}
	p.Init([]uint64{10})

	params := p.Params()
	if len(params) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(params))
	}

	for _, v := range params {
		if _, ok := v["a"]; !ok {
			t.Fatalf("missing a")
		}
		if _, ok := v["b"]; !ok {
			t.Fatalf("missing b")
		}
		if _, ok := v["c"]; !ok {
			t.Fatalf("missing c")
		}
	}
}
