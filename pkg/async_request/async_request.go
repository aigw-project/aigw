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
	"sync"
	"time"

	"mosn.io/htnn/api/pkg/filtermanager/api"
)

type Task struct {
	TraceId string
	Method  string
	URL     string
	Body    []byte
	Timeout time.Duration
	HashKey string
}

type AsyncRequestHandler interface {
	HandleRequest(ctx context.Context, task Task) error
}

type Config struct {
	QueueSize      int
	WorkerCount    int
	MaxRetries     int
	DefaultTimeout time.Duration
}

type AsyncQueue struct {
	config   Config
	tasks    chan Task
	handler  AsyncRequestHandler
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	isClosed bool
}

func NewAsyncQueue(cfg Config, handler AsyncRequestHandler) *AsyncQueue {
	ctx, cancel := context.WithCancel(context.Background())

	aq := &AsyncQueue{
		config:  cfg,
		tasks:   make(chan Task, cfg.QueueSize),
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}

	aq.wg.Add(cfg.WorkerCount)
	for i := 0; i < cfg.WorkerCount; i++ {
		go aq.worker(i)
	}

	return aq
}

func (aq *AsyncQueue) Dispatch(task Task) error {
	aq.mu.RLock()
	defer aq.mu.RUnlock()

	if aq.isClosed {
		return errors.New("queue is closed")
	}

	if task.Timeout == 0 {
		task.Timeout = aq.config.DefaultTimeout
	}

	select {
	case aq.tasks <- task:
		api.LogDebugf("[TraceID: %s] enqueued task to AsyncQueue, method: %s, url: %s, timeout: %v, hashKey: %s", task.TraceId, task.Method, task.URL, task.Timeout, task.HashKey)
		return nil
	case <-aq.ctx.Done():
		return errors.New("queue context canceled")
	default:
		return fmt.Errorf("queue is full, length: %d", len(aq.tasks))
	}
}

func (aq *AsyncQueue) worker(id int) {
	defer aq.wg.Done()

	for {
		select {
		case task := <-aq.tasks:
			startTime := time.Now()
			var err error
			for attempt := 0; attempt <= aq.config.MaxRetries; attempt++ {
				err = aq.handler.HandleRequest(aq.ctx, task)
				if err == nil {
					break
				}
				api.LogInfof("[TraceID: %s] worker attempt: %d, error: %v", task.TraceId, attempt+1, err)
				if attempt < aq.config.MaxRetries {
					time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
				}
			}
			endTime := time.Now()
			logStatus(id, task, startTime, endTime, err)

		case <-aq.ctx.Done():
			return
		}
	}
}

func logStatus(id int, task Task, start, end time.Time, err error) {
	status := "SUCCESS"
	if err != nil {
		status = "FAIL"
	}
	duration := end.Sub(start)
	if duration > 10*time.Millisecond {
		api.LogWarnf("[TraceID: %s] worker:%d, method:%s, url:%s, start:%s, end:%s, duration: %v, status: %s, error: %v",
			task.TraceId,
			id,
			task.Method,
			task.URL,
			start.Format("2006-01-02 15:04:05.000"),
			end.Format("2006-01-02 15:04:05.000"),
			duration.Round(time.Millisecond),
			status,
			err)
	}
}

func (aq *AsyncQueue) Shutdown() {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	if aq.isClosed {
		return
	}

	aq.isClosed = true
	aq.cancel()
	aq.wg.Wait()
	close(aq.tasks)
}
