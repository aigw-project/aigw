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

package async_log

import (
	"io"
	"os"
	"strconv"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"
	"mosn.io/htnn/api/pkg/filtermanager/api"
)

const DefaultLogPath = "/home/admin/logs/llm.log"

var (
	logger     *AsyncLogger
	loggerOnce sync.Once
)

type AsyncLogger struct {
	logQueue chan []byte
	stopChan chan struct{}
	logger   io.Writer
}

func GetAsyncLoggerInstance(path string, queueSize int) *AsyncLogger {
	loggerOnce.Do(func() {
		if path == "" {
			path = DefaultLogPath
		}
		logger = createAsyncLogger(path, queueSize)
	})
	return logger
}

func createAsyncLogger(path string, queueSize int) *AsyncLogger {
	l := getLogger(path)
	al := &AsyncLogger{
		logQueue: make(chan []byte, queueSize),
		stopChan: make(chan struct{}),
		logger:   l,
	}
	go al.run()
	return al
}

func (al *AsyncLogger) Write(p []byte) {
	select {
	case al.logQueue <- p:
	default:
		api.LogErrorf("log queue is full, dropping log")
	}
}

func (al *AsyncLogger) run() {
	for {
		select {
		case log := <-al.logQueue:
			_, err := al.logger.Write(log)
			if err != nil {
				api.LogErrorf("async log write failed: %v", err)
			}
		case <-al.stopChan:
			return
		}
	}
}

func getLogger(path string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    getEnvInt("AIGW_LLM_LOGSIZE", 100), // 按大小切割不会存在软链接的问题
		MaxBackups: getEnvInt("AIGW_LLM_LOGBACKUPS", 100),
		MaxAge:     getEnvInt("AIGW_LLM_MAXAGE", 7),
		Compress:   getEnvBool("AIGW_LLM_COMPRESS", false),
	}
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultValue
}
