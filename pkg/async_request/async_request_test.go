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

package async_request

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	_ "mosn.io/htnn/api/plugins/tests/pkg/envoy"
)

type MockHandler struct {
	mu          sync.Mutex
	callCount   int
	returnError bool
	block       bool
	delay       time.Duration
}

func (m *MockHandler) HandleRequest(ctx context.Context, task Task) error {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.block {
		<-ctx.Done()
		return ctx.Err()
	}

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.returnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *MockHandler) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestDispatchWhenClosed(t *testing.T) {
	handler := &MockHandler{}
	queue := NewAsyncQueue(Config{
		QueueSize:   1,
		WorkerCount: 1,
	}, handler)
	queue.Shutdown()

	err := queue.Dispatch(Task{Method: "GET", URL: "/test"})
	if err == nil || !strings.Contains(err.Error(), "closed") {
		t.Errorf("Expected closed error, got: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	handler := &MockHandler{block: true}
	queue := NewAsyncQueue(Config{
		QueueSize:   2,
		WorkerCount: 1,
	}, handler)

	for i := 0; i < 2; i++ {
		if err := queue.Dispatch(Task{Method: "GET", URL: "/test"}); err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}
	}

	err := queue.Dispatch(Task{Method: "GET", URL: "/test"})
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Errorf("Expected queue full error, got: %v", err)
	}

	queue.Shutdown()
}

func TestRetryMechanism(t *testing.T) {
	maxRetries := 3
	handler := &MockHandler{returnError: true}
	queue := NewAsyncQueue(Config{
		QueueSize:   1,
		WorkerCount: 1,
		MaxRetries:  maxRetries,
	}, handler)

	if err := queue.Dispatch(Task{Method: "POST", URL: "/api"}); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	queue.Shutdown()

	expectedCalls := maxRetries + 1
	if handler.GetCallCount() != expectedCalls {
		t.Errorf("Expected %d calls, got %d", expectedCalls, handler.GetCallCount())
	}
}

func TestTimeoutHandling(t *testing.T) {
	handler := &MockHandler{delay: 200 * time.Millisecond}
	queue := NewAsyncQueue(Config{
		QueueSize:      1,
		WorkerCount:    1,
		DefaultTimeout: 100 * time.Millisecond,
	}, handler)

	if err := queue.Dispatch(Task{Method: "GET", URL: "/slow"}); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	queue.Shutdown()

	if handler.GetCallCount() != 1 {
		t.Errorf("Expected 1 call, got %d", handler.GetCallCount())
	}
}

func TestShutdownDuringProcessing(t *testing.T) {
	handler := &MockHandler{block: true}
	queue := NewAsyncQueue(Config{
		QueueSize:   1,
		WorkerCount: 1,
	}, handler)

	if err := queue.Dispatch(Task{Method: "GET", URL: "/block"}); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	go queue.Shutdown()

	done := make(chan struct{})
	go func() {
		queue.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown timed out")
	}
}

func TestConcurrentDispatch(t *testing.T) {
	handler := &MockHandler{}
	queue := NewAsyncQueue(Config{
		QueueSize:   100,
		WorkerCount: 10,
	}, handler)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			task := Task{
				Method: "POST",
				URL:    fmt.Sprintf("/api/%d", n),
			}
			if err := queue.Dispatch(task); err != nil {
				t.Errorf("Dispatch failed: %v", err)
			}
		}(i)
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	queue.Shutdown()

	if handler.GetCallCount() != 100 {
		t.Errorf("Expected 100 processed tasks, got %d", handler.GetCallCount())
	}
}
