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

package circuitbreaker

import (
	"sync"
	"time"

	"github.com/envoyproxy/envoy/contrib/golang/common/go/api"
)

type CircuitBreakerConfig struct {
	MaxFailures      int
	CooldownPeriod   time.Duration
	HalfOpenRequests int
}

type CircuitBreaker interface {
	Allow() bool
	RecordSuccess()
	RecordFailure()
	State() string
}

type countingCircuitBreaker struct {
	mu              sync.Mutex
	config          CircuitBreakerConfig
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	state           string // closed, open, half-open
}

func NewCircuitBreaker(config CircuitBreakerConfig) CircuitBreaker {
	return &countingCircuitBreaker{
		config: config,
		state:  "closed",
	}
}

func (cb *countingCircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	api.LogDebugf("CircuitBreaker Allow: %d, %s", cb.failureCount, cb.state)
	switch cb.state {
	case "open":
		if time.Since(cb.lastFailureTime) > cb.config.CooldownPeriod {
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		return cb.successCount < cb.config.HalfOpenRequests
	default:
		return true
	}
}

func (cb *countingCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "closed":
		cb.failureCount = 0
	case "half-open":
		cb.successCount++
		if cb.successCount >= cb.config.HalfOpenRequests {
			cb.state = "closed"
			cb.resetCounters()
		}
	}
}

func (cb *countingCircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()
	api.LogDebugf("CircuitBreaker RecordFailure: %d, %s", cb.failureCount, cb.state)

	if cb.state == "closed" && cb.failureCount >= cb.config.MaxFailures {
		cb.state = "open"
		cb.resetCounters()
		return
	}

	if cb.state == "half-open" {
		cb.state = "open"
		cb.resetCounters()
	}
}

func (cb *countingCircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *countingCircuitBreaker) resetCounters() {
	cb.failureCount = 0
	cb.successCount = 0
}
