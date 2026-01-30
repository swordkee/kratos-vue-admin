package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// OperationRecord returns a middleware for recording API operations.
func OperationRecord(opRecordsCase *biz.SysLogsUseCase) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Get HTTP request from context using Kratos API
			var httpReq *http.Request
			if kratosReq, ok := http.RequestFromServerContext(ctx); ok {
				httpReq = kratosReq
			}

			// Capture request body
			var reqBody []byte
			if httpReq != nil && httpReq.Method != "GET" {
				bodyBytes, err := io.ReadAll(httpReq.Body)
				if err == nil {
					reqBody = bodyBytes
					// Restore the body for later handlers
					httpReq.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}

			// Get user ID from context (set by auth middleware)
			userID := int64(0)
			if userIDStr := ctx.Value("user-id"); userIDStr != nil {
				if id, err := strconv.ParseInt(userIDStr.(string), 10, 64); err == nil {
					userID = id
				}
			}

			// Record start time for latency calculation
			startTime := time.Now()

			// Create operation record with initial values
			record := &model.SysLogs{
				IP:     getClientIP(httpReq),
				Method: getMethod(httpReq),
				Path:   getPath(httpReq),
				Agent:  getUserAgent(httpReq),
				Body:   truncateBody(string(reqBody)),
				UserID: userID,
			}

			// Call the handler
			reply, err = handler(ctx, req)

			// Calculate latency after handler completes
			latency := time.Since(startTime)
			record.Latency = int64(latency)

			// Get error message if any
			if err != nil {
				record.ErrorMessage = err.Error()
			}

			// Capture response body
			if reply != nil {
				respBytes, jsonErr := json.Marshal(reply)
				if jsonErr == nil {
					record.Resp = truncateBody(string(respBytes))
				}
			}

			// Save operation record asynchronously to avoid blocking
			// Use context.Background() instead of request ctx because:
			// 1. Request context is canceled when HTTP response is sent
			// 2. Database transaction may already be committed/rolled back
			// 3. Async operation needs independent context that won't be canceled
			go func() {
				bgCtx := context.Background()
				saveErr := opRecordsCase.CreateOperationRecord(bgCtx, record)
				if saveErr != nil {
					// In production, use a proper logger here
				}
			}()

			return reply, err
		}
	}
}

// getClientIP extracts the client IP from the HTTP request
func getClientIP(req *http.Request) string {
	if req == nil {
		return ""
	}
	forwarded := req.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		if idx := bytes.IndexByte([]byte(forwarded), ','); idx != -1 {
			return forwarded[:idx]
		}
		return forwarded
	}
	if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	if req.RemoteAddr != "" {
		if idx := bytes.IndexByte([]byte(req.RemoteAddr), ':'); idx != -1 {
			return req.RemoteAddr[:idx]
		}
		return req.RemoteAddr
	}
	return ""
}

func getMethod(req *http.Request) string {
	if req == nil {
		return ""
	}
	return req.Method
}

func getPath(req *http.Request) string {
	if req == nil {
		return ""
	}
	return req.URL.Path
}

func getUserAgent(req *http.Request) string {
	if req == nil {
		return ""
	}
	return req.Header.Get("User-Agent")
}

const maxBodyLength = 4096

func truncateBody(body string) string {
	if len(body) > maxBodyLength {
		return body[:maxBodyLength] + "...[truncated]"
	}
	return body
}
