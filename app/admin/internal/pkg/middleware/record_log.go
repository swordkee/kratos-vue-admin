package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// LogConfig 日志记录配置
type LogConfig struct {
	// EnableReadLog 是否记录读操作(GET/HEAD/OPTIONS)，默认false
	EnableReadLog bool
	// EnableWriteLog 是否记录写操作(POST/PUT/DELETE/PATCH)，默认true
	EnableWriteLog bool
	// MaxBodyLength 请求/响应体最大长度
	MaxBodyLength int
}

// DefaultLogConfig 返回默认日志配置
// 默认只记录写操作，不记录读操作
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		EnableReadLog:  false,
		EnableWriteLog: true,
		MaxBodyLength:  4096,
	}
}

// isReadMethod 判断是否为读操作
func isReadMethod(method string) bool {
	method = strings.ToUpper(method)
	return method == "GET" || method == "HEAD" || method == "OPTIONS"
}

// isWriteMethod 判断是否为写操作
func isWriteMethod(method string) bool {
	method = strings.ToUpper(method)
	return method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH"
}

// shouldRecord 根据配置和方法类型判断是否应该记录日志
func shouldRecord(method string, config *LogConfig) bool {
	if config == nil {
		config = DefaultLogConfig()
	}

	method = strings.ToUpper(method)

	// 读操作
	if isReadMethod(method) {
		return config.EnableReadLog
	}

	// 写操作
	if isWriteMethod(method) {
		return config.EnableWriteLog
	}

	// 其他方法默认记录
	return true
}

// OperationRecord returns a middleware for recording API operations.
// 默认只记录写操作(POST/PUT/DELETE/PATCH)，读操作(GET/HEAD/OPTIONS)不记录
func OperationRecord(opRecordsCase *biz.SysLogsUseCase) middleware.Middleware {
	return OperationRecordWithConfig(opRecordsCase, DefaultLogConfig())
}

// OperationRecordWithConfig returns a middleware for recording API operations with custom config.
// 支持自定义配置，可控制是否记录读操作和写操作
func OperationRecordWithConfig(opRecordsCase *biz.SysLogsUseCase, config *LogConfig) middleware.Middleware {
	if config == nil {
		config = DefaultLogConfig()
	}

	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			// Get HTTP request from context using Kratos API
			var httpReq *http.Request
			if kratosReq, ok := http.RequestFromServerContext(ctx); ok {
				httpReq = kratosReq
			}

			method := getMethod(httpReq)

			// 根据配置判断是否记录此请求
			if !shouldRecord(method, config) {
				// 不记录日志，直接执行handler
				return handler(ctx, req)
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
				Method: method,
				Path:   getPath(httpReq),
				Agent:  getUserAgent(httpReq),
				Body:   truncateBody(string(reqBody), config.MaxBodyLength),
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
					record.Resp = truncateBody(string(respBytes), config.MaxBodyLength)
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

const defaultMaxBodyLength = 4096

// truncateBody 截断请求/响应体
// 如果 maxLength <= 0，则使用默认值
func truncateBody(body string, maxLength int) string {
	if maxLength <= 0 {
		maxLength = defaultMaxBodyLength
	}
	if len(body) > maxLength {
		return body[:maxLength] + "...[truncated]"
	}
	return body
}
