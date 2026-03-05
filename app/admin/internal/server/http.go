package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"

	"github.com/go-kratos/kratos/v2/transport/http/pprof"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/middleware"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/handlers"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	v1 "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	adminV1 "github.com/swordkee/kratos-vue-admin/app/admin/internal/service/admin"
)

// CustomRequestDecoder 自定义请求解码器，使用标准 encoding/json
func CustomRequestDecoder(r *stdhttp.Request, v interface{}) error {
	// 检查 Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	// 恢复 body，以便后续中间件可以再次读取
	r.Body = io.NopCloser(bytes.NewBuffer(data))

	if len(data) == 0 {
		return nil
	}

	// 直接解码到目标结构体
	return json.Unmarshal(data, v)
}

// convertStringToNumber 递归转换 map 中的字符串数字为数值类型
func convertStringToNumber(m map[string]interface{}) {
	for key, value := range m {
		switch v := value.(type) {
		case string:
			// 尝试转换为整数
			if intVal, err := parseInt(v); err == nil {
				m[key] = intVal
			}
		case map[string]interface{}:
			convertStringToNumber(v)
		case []interface{}:
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					convertStringToNumber(itemMap)
				} else if str, ok := item.(string); ok {
					if intVal, err := parseInt(str); err == nil {
						v[i] = intVal
					}
				}
			}
		}
	}
}

// parseInt 尝试将字符串解析为整数
func parseInt(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// extractTraceId 从 context 中提取 traceId
func extractTraceId(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	// 尝试从 context 中提取 traceId
	// Kratos tracing 中间件会将 traceId 存储在 context 中
	if span := ctx.Value("x-b3-traceid"); span != nil {
		if s, ok := span.(string); ok {
			return s
		}
	}
	// 尝试其他可能的 key
	if span := ctx.Value("traceId"); span != nil {
		if s, ok := span.(string); ok {
			return s
		}
	}
	return ""
}

// jsonMarshal marshals CommonReply with @type cleanup
func jsonMarshal(res *pb.CommonReply) ([]byte, error) {
	newProto := protojson.MarshalOptions{EmitUnpopulated: true}
	output, err := newProto.Marshal(res)
	if err != nil {
		return nil, err
	}

	var stuff map[string]any
	if err := json.Unmarshal(output, &stuff); err != nil {
		return nil, err
	}

	if stuff["data"] != nil {
		delete(stuff["data"].(map[string]any), "@type")
	}
	return json.MarshalIndent(stuff, "", "  ")
}

func EncoderResponse() http.EncodeResponseFunc {
	return func(w stdhttp.ResponseWriter, request *stdhttp.Request, i interface{}) error {
		// 从 context 中提取 traceId
		traceId := ""
		if request != nil {
			traceId = extractTraceId(request.Context())
		}

		resp := &pb.CommonReply{
			Code:    200,
			Message: "",
			TraceId: traceId,
		}
		var data []byte
		var err error
		if m, ok := i.(proto.Message); ok {
			payload, err := anypb.New(m)
			if err != nil {
				return err
			}
			resp.Data = payload
			data, err = jsonMarshal(resp)
			if err != nil {
				return err
			}
		} else {
			dataMap := map[string]interface{}{
				"code":    200,
				"message": "",
				"traceId": traceId,
				"data":    i,
			}
			data, err = json.Marshal(dataMap)
			if err != nil {
				return err
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		if err != nil {
			return err
		}
		return nil
	}
}

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	s *conf.Auth,
	casbinRepo admin.CasbinRuleRepo,
	userRepo admin.SysUserRepo,
	logger log.Logger,
	sysUserService *adminV1.SysUserService,
	apiService *adminV1.ApiService,
	deptService *adminV1.DeptService,
	opRecordsCase *biz.SysLogsUseCase,
	opRecordsService *adminV1.SysLogsService,
	menusService *adminV1.MenusService,
	postService *adminV1.PostService,
	dictTypeService *adminV1.DictTypeService,
	dictDataService *adminV1.DictDataService,
	roleService *adminV1.RolesService,
) *http.Server {
	// 构建日志中间件配置
	logMiddlewareConfig := middleware.DefaultLogConfig()

	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
			middleware.OperationRecordWithConfig(opRecordsCase, logMiddlewareConfig),
			middleware.Auth(s, casbinRepo, userRepo),
		),
		http.Filter(handlers.CORS(
			handlers.AllowedHeaders([]string{"Accept", "Accept-Language", "Content-Language", "Origin", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"}),
			handlers.AllowedOrigins([]string{"*"}),
			handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"}),
			handlers.AllowCredentials(),
		)),
		http.ResponseEncoder(EncoderResponse()),
		http.RequestDecoder(CustomRequestDecoder),
	}

	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterSysUserHTTPServer(srv, sysUserService)
	v1.RegisterApiHTTPServer(srv, apiService)
	v1.RegisterDeptHTTPServer(srv, deptService)
	v1.RegisterLogsServiceHTTPServer(srv, opRecordsService)
	v1.RegisterMenusHTTPServer(srv, menusService)
	v1.RegisterSysPostHTTPServer(srv, postService)
	v1.RegisterDictTypeHTTPServer(srv, dictTypeService)
	v1.RegisterDictDataHTTPServer(srv, dictDataService)
	v1.RegisterRolesHTTPServer(srv, roleService)

	// 上传文件的路由
	r := srv.Route("/")
	r.POST("/system/user/avatar", func(ctx http.Context) error {
		http.SetOperation(ctx, "/api.admin.v1.Sysuser/UpdateAvatar")
		h := ctx.Middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, sysUserService.UpdateAvatar(ctx)
		})

		if _, err := h(ctx, nil); err != nil {
			return err
		}
		return ctx.Result(200, &struct{}{})
	})
	r.POST("/file/upload", func(ctx http.Context) error {
		http.SetOperation(ctx, "/api.admin.v1.Sysuser/UploadFile")
		url, err := sysUserService.UploadFile(ctx)
		if err != nil {
			return err
		}
		rep := make(map[string]string)
		rep["url"] = url
		return ctx.Result(200, url)
	})

	srv.Handle("/debug/pprof/", pprof.NewHandler())

	return srv
}
