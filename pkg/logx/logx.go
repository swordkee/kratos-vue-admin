// Package logx 提供 V3 风格结构化日志包装，底层使用 log/slog。
//
// 基础用法：
//
//	logx.Infow(ctx, "User login success", "userID", id, "ip", ip)
//	logx.Errorf(ctx, "query failed: userID=%d, err=%v", uid, err)
//
// OpenTelemetry 集成：
//
//	import (
//	    "log/slog"
//	    "github.com/go-kratos/kratos/contrib/otel/v3/log" // 仅 Kratos v3
//	)
//
//	handler := otel.NewHandler("my-service")
//	logx.SetDefault(slog.New(handler))
package logx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// SetDefault 替换全局 slog.Logger（用于注入 OTEL handler 等自定义实现）。
// 不调用则默认使用 slog.Default()。
func SetDefault(logger *slog.Logger) {
	slog.SetDefault(logger)
}

// Logger 可嵌入 struct 的日志实例，持有 *slog.Logger 并附加脱敏能力。
type Logger struct {
	base *slog.Logger
	d    *Desensitizer
}

// Option 日志配置选项
type Option func(*Logger)

// WithDesensitize 启用日志脱敏（自动隐藏手机号、身份证、银行卡等）
func WithDesensitize() Option {
	return func(l *Logger) { l.d = NewDesensitizer(l.base) }
}

// NewLogger 创建日志实例，base 为底层 slog.Logger。
func NewLogger(base *slog.Logger, opts ...Option) *Logger {
	if base == nil {
		base = slog.Default()
	}
	l := &Logger{base: base}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Slog 返回底层 *slog.Logger，可直接传给 kratos.Logger() 等。
func (l *Logger) Slog() *slog.Logger {
	return l.base
}

// With 返回一个附加了 attrs 的新 Logger（用于模块/服务标签，如
// logger.With(slog.String("module", "biz/device_init"))）。原 Logger 不变。
func (l *Logger) With(attrs ...slog.Attr) *Logger {
	args := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		args = append(args, a.Key, a.Value.Any())
	}
	sub := NewLogger(l.base.With(args...))
	sub.d = l.d
	return sub
}

func (l *Logger) Debugw(ctx context.Context, msg string, args ...any) {
	if l.d != nil {
		msg = l.d.Desensitize(msg)
		args = l.d.desensitizeKeyvals(args)
	}
	l.base.DebugContext(ctx, msg, args...)
}
func (l *Logger) Infow(ctx context.Context, msg string, args ...any) {
	if l.d != nil {
		msg = l.d.Desensitize(msg)
		args = l.d.desensitizeKeyvals(args)
	}
	l.base.InfoContext(ctx, msg, args...)
}
func (l *Logger) Warnw(ctx context.Context, msg string, args ...any) {
	if l.d != nil {
		msg = l.d.Desensitize(msg)
		args = l.d.desensitizeKeyvals(args)
	}
	l.base.WarnContext(ctx, msg, args...)
}
func (l *Logger) Errorw(ctx context.Context, msg string, args ...any) {
	if l.d != nil {
		msg = l.d.Desensitize(msg)
		args = l.d.desensitizeKeyvals(args)
	}
	l.base.ErrorContext(ctx, msg, args...)
}

// Debug 等价于 Debugw（兼容旧版 logger.Logger 接口）
func (l *Logger) Debug(msg string, args ...any) { l.Debugw(context.Background(), msg, args...) }

// Info 等价于 Infow（兼容旧版 logger.Logger 接口）
func (l *Logger) Info(msg string, args ...any) { l.Infow(context.Background(), msg, args...) }

// Warn 等价于 Warnw（兼容旧版 logger.Logger 接口）
func (l *Logger) Warn(msg string, args ...any) { l.Warnw(context.Background(), msg, args...) }

// Error 等价于 Errorw（兼容旧版 logger.Logger 接口）
func (l *Logger) Error(msg string, args ...any) { l.Errorw(context.Background(), msg, args...) }

// Fatal logs at error level and then panics (rather than calling os.Exit),
// so deferred cleanup in the same goroutine still runs. Callers that need
// process termination should recover at main and call os.Exit explicitly.
func (l *Logger) Fatal(msg string, args ...any) {
	l.Errorw(context.Background(), msg, args...)
	panic("logx.Fatal: " + msg)
}

// InfoContext 等价于 Infow（兼容 slog 命名）
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.Infow(ctx, msg, args...)
}

// WarnContext 等价于 Warnw（兼容 slog 命名）
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.Warnw(ctx, msg, args...)
}

// DebugContext 等价于 Debugw（兼容 slog 命名）
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.Debugw(ctx, msg, args...)
}

// ErrorContext 等价于 Errorw（兼容 slog 命名）
func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.Errorw(ctx, msg, args...)
}

func (l *Logger) Debugf(ctx context.Context, f string, args ...any) {
	if l.d != nil {
		f = l.d.Desensitize(f)
		args = l.d.desensitizeArgs(args)
	}
	l.base.DebugContext(ctx, fmt.Sprintf(f, args...))
}
func (l *Logger) Infof(ctx context.Context, f string, args ...any) {
	if l.d != nil {
		f = l.d.Desensitize(f)
		args = l.d.desensitizeArgs(args)
	}
	l.base.InfoContext(ctx, fmt.Sprintf(f, args...))
}
func (l *Logger) Warnf(ctx context.Context, f string, args ...any) {
	if l.d != nil {
		f = l.d.Desensitize(f)
		args = l.d.desensitizeArgs(args)
	}
	l.base.WarnContext(ctx, fmt.Sprintf(f, args...))
}
func (l *Logger) Errorf(ctx context.Context, f string, args ...any) {
	if l.d != nil {
		f = l.d.Desensitize(f)
		args = l.d.desensitizeArgs(args)
	}
	l.base.ErrorContext(ctx, fmt.Sprintf(f, args...))
}

// Infow 结构化 INFO
func Infow(ctx context.Context, msg string, args ...any) {
	slog.Default().InfoContext(ctx, msg, args...)
}

// Errorw 结构化 ERROR
func Errorw(ctx context.Context, msg string, args ...any) {
	slog.Default().ErrorContext(ctx, msg, args...)
}

// Warnw 结构化 WARN
func Warnw(ctx context.Context, msg string, args ...any) {
	slog.Default().WarnContext(ctx, msg, args...)
}

// Debugw 结构化 DEBUG
func Debugw(ctx context.Context, msg string, args ...any) {
	slog.Default().DebugContext(ctx, msg, args...)
}

// Infof 格式化 INFO
func Infof(ctx context.Context, format string, args ...any) {
	slog.Default().InfoContext(ctx, fmt.Sprintf(format, args...))
}

// Errorf 格式化 ERROR
func Errorf(ctx context.Context, format string, args ...any) {
	slog.Default().ErrorContext(ctx, fmt.Sprintf(format, args...))
}

// Warnf 格式化 WARN
func Warnf(ctx context.Context, format string, args ...any) {
	slog.Default().WarnContext(ctx, fmt.Sprintf(format, args...))
}

// Debugf 格式化 DEBUG
func Debugf(ctx context.Context, format string, args ...any) {
	slog.Default().DebugContext(ctx, fmt.Sprintf(format, args...))
}

// InitJSON 快速初始化 JSON 格式日志（开发环境）
func InitJSON(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))
}

// InitText 快速初始化文本格式日志（生产环境默认）
func InitText(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))
}

// HandlerConfig 构建 slog.Handler 的配置
type HandlerConfig struct {
	Level       string // "debug", "info", "warn", "error"
	OutputPaths string // 逗号分隔: "stdout", "stderr", 文件路径
}

// BuildHandler 根据配置构建 slog.Handler，支持多输出（stdout + 文件）。
func BuildHandler(cfg HandlerConfig) slog.Handler {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var writers []io.Writer
	for p := range strings.SplitSeq(cfg.OutputPaths, ",") {
		p = strings.TrimSpace(p)
		switch p {
		case "stdout":
			writers = append(writers, os.Stdout)
		case "stderr":
			writers = append(writers, os.Stderr)
		case "":
		default:
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				writers = append(writers, f)
			} else {
				fmt.Fprintf(os.Stderr, "[logx] WARNING: cannot open log file %q: %v, falling back to stdout\n", p, err)
				writers = append(writers, os.Stdout)
			}
		}
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	w := writers[0]
	if len(writers) > 1 {
		w = io.MultiWriter(writers...)
	}
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "ts"
				if a.Value.Kind() == slog.KindTime {
					a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02T15:04:05.000Z0700"))
				}
			case slog.SourceKey:
				if src, ok := a.Value.Any().(*slog.Source); ok {
					a = slog.String("caller", fmt.Sprintf("%s:%d", trimCaller(src.File), src.Line))
				}
			}
			return a
		},
	}
	return &traceHandler{inner: slog.NewJSONHandler(w, opts)}
}

// trimCaller 截取调用文件为最后两级目录 + 文件名，与 zap short caller 保持一致。
func trimCaller(path string) string {
	idx := strings.LastIndexByte(path, '/')
	if idx < 0 {
		return path
	}
	if idx2 := strings.LastIndexByte(path[:idx], '/'); idx2 >= 0 {
		return path[idx2+1:]
	}
	return path
}

// traceHandler 从 ctx 中提取 trace_id / span_id 注入日志（轻量版，不写 span event）。
type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.Int("severity_number", severityNumber(r.Level)))
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

const (
	LevelDPanic = slog.Level(10)
	LevelPanic  = slog.Level(12)
	LevelFatal  = slog.Level(14)
)

func severityNumber(level slog.Level) int {
	switch {
	case level < slog.LevelInfo:
		return 5 // DEBUG
	case level < slog.LevelWarn:
		return 9 // INFO
	case level < slog.LevelError:
		return 13 // WARN
	case level < LevelDPanic:
		return 17 // ERROR
	case level < LevelPanic:
		return 18 // DPANIC
	case level < LevelFatal:
		return 19 // PANIC
	default:
		return 21 // FATAL
	}
}
func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}

// ======== 日志脱敏 ========

// SensitiveField 敏感字段配置
type SensitiveField struct {
	Name        string
	Pattern     *regexp.Regexp
	ReplaceWith string
}

// Desensitizer 日志脱敏器
type Desensitizer struct {
	fields []SensitiveField
	logger *slog.Logger
}

var (
	phoneRegex    = regexp.MustCompile(`1[3-9]\d{9}`)
	idCardRegex   = regexp.MustCompile(`[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`)
	bankCardRegex = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
	emailRegex    = regexp.MustCompile(`[\w\.-]+@[\w\.-]+\.\w+`)
	passwordRegex = regexp.MustCompile(`(?i)(password|passwd|pwd|secret|key|token|auth|credential|apikey|api_key|access_key|access_token|refresh_token|private_key|public_key|sign_key|encrypt_key|client_secret|master_secret|hmac|salt|nonce|iv|session)(_?\w*)?\s*[=:]\s*["']?[\w\-\./\+=]+["']?`)
	jwtRegex      = regexp.MustCompile(`eyJ[\w\-_]+\.[\w\-_]+\.[\w\-_]+`)
	ipRegex       = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
)

// NewDesensitizer 创建日志脱敏器
func NewDesensitizer(logger *slog.Logger) *Desensitizer {
	return &Desensitizer{
		fields: []SensitiveField{
			{Name: "phone", Pattern: phoneRegex, ReplaceWith: "1*** **** ***"},
			{Name: "id_card", Pattern: idCardRegex, ReplaceWith: "******************"},
			{Name: "bank_card", Pattern: bankCardRegex, ReplaceWith: "**** **** **** ****"},
			{Name: "email", Pattern: emailRegex, ReplaceWith: "m***@***.com"},
			{Name: "password", Pattern: passwordRegex, ReplaceWith: "***"},
			{Name: "jwt", Pattern: jwtRegex, ReplaceWith: "eyJ***"},
			{Name: "ip", Pattern: ipRegex, ReplaceWith: "***.***.***.***"},
		},
		logger: logger,
	}
}

// Desensitize 脱敏字符串
func (d *Desensitizer) Desensitize(input string) string {
	result := input
	for _, field := range d.fields {
		result = field.Pattern.ReplaceAllString(result, field.ReplaceWith)
	}
	return result
}

func (d *Desensitizer) desensitizeArgs(args []any) []any {
	result := make([]any, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			result[i] = d.Desensitize(str)
		} else {
			result[i] = arg
		}
	}
	return result
}

func (d *Desensitizer) isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"password", "passwd", "pwd", "secret", "key", "token", "auth",
		"credential", "api_key", "apikey", "access_token", "refresh_token",
		"private_key", "public_key", "sign_key", "encrypt_key",
		"access_key", "accesskey", "access_secret", "secret_key", "secretkey",
		"client_secret", "clientsecret", "master_secret", "mastersecret",
		"hmac", "hmac_secret", "hmac_secret", "sign_secret", "signsecret",
		"salt", "iv", "nonce", "session", "cookie", "jwt", "bearer",
		"admin_token", "admintoken", "app_secret", "app_id_secret",
		"aliyun_ak", "aliyun_sk", "oss_access_key", "oss_access_secret",
		"tushare_token", "jisuapi_token", "upush_master_secret",
		"db_password", "dbpass", "mysql_password", "rds_password",
		"dev_bypass", "dev_bypass_code", "bypass_code",
	}
	for _, s := range sensitiveKeys {
		if strings.Contains(key, s) {
			return true
		}
	}
	return false
}

func (d *Desensitizer) desensitizeKeyvals(keyvals []any) []any {
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "")
	}
	result := make([]any, len(keyvals))
	for i := 0; i < len(keyvals); i += 2 {
		result[i] = keyvals[i]
		value := keyvals[i+1]
		if keyStr, ok := keyvals[i].(string); ok && d.isSensitiveKey(strings.ToLower(keyStr)) {
			// 敏感 key：整体替换为 "***"，不做正则匹配，确保任何形状的密钥都不落日志
			if str, ok := value.(string); ok && str != "" {
				// JWT 单独掩码（保留 eyJ 前缀便于识别），其余敏感值保留尾 4 位便于排查
				if strings.HasPrefix(str, "eyJ") {
					result[i+1] = "eyJ***"
				} else if len(str) > 8 {
					result[i+1] = "***" + str[len(str)-4:]
				} else {
					result[i+1] = "***"
				}
			} else {
				result[i+1] = "***"
			}
		} else if str, ok := value.(string); ok {
			result[i+1] = d.Desensitize(str)
		} else {
			result[i+1] = value
		}
	}
	return result
}

// ======== 脱敏辅助函数 ========

// MaskString 按规则脱敏字符串
func MaskString(s string, keepLeft, keepRight int, maskChar string) string {
	if len(s) <= keepLeft+keepRight {
		return strings.Repeat(maskChar, len(s))
	}
	return s[:keepLeft] + strings.Repeat(maskChar, len(s)-keepLeft-keepRight) + s[len(s)-keepRight:]
}

// MaskPhone 脱敏手机号
func MaskPhone(phone string) string {
	if len(phone) == 11 {
		return phone[:3] + "****" + phone[7:]
	}
	return MaskString(phone, 3, 4, "*")
}

// MaskEmail 脱敏邮箱
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***.com"
	}
	return parts[0][:1] + strings.Repeat("*", len(parts[0])-1) + "@" + parts[1]
}

// MaskIDCard 脱敏身份证号
func MaskIDCard(idCard string) string {
	if len(idCard) >= 18 {
		return idCard[:6] + "********" + idCard[14:]
	}
	return strings.Repeat("*", len(idCard))
}

// MaskBankCard 脱敏银行卡号
func MaskBankCard(cardNo string) string {
	cleaned := strings.ReplaceAll(strings.ReplaceAll(cardNo, " ", ""), "-", "")
	if len(cleaned) >= 16 {
		return "**** **** **** " + cleaned[len(cleaned)-4:]
	}
	return MaskString(cardNo, 4, 4, "*")
}
