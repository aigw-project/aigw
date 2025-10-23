package common

import (
	"io"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var expectedGrpcFailureMessages = []string{
	"client disconnected",
	"error reading from server: EOF",
	"transport is closing",
}

func containsExpectedMessage(msg string) bool {
	for _, m := range expectedGrpcFailureMessages {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// IsExpectedGRPCError checks a gRPC error code and determines whether it is an expected error when
// things are operating normally. This is basically capturing when the client disconnects.
func IsExpectedGRPCError(err error) bool {
	if err == io.EOF {
		return true
	}

	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.Canceled || s.Code() == codes.DeadlineExceeded {
			return true
		}
		if s.Code() == codes.Unavailable && containsExpectedMessage(s.Message()) {
			return true
		}
	}
	// If this is not a gRPCStatus we should just error message.
	if strings.Contains(err.Error(), "stream terminated by RST_STREAM with error code: NO_ERROR") {
		return true
	}

	return false
}
