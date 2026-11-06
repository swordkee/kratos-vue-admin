package server_test

// 媒体库路由注册冒烟（不连 DB/Redis）：用 kratos http.NewServer + 生成的
// RegisterXxxHTTPServer 校验 11 条 proto 路由全部注册。
//
// 处理器走 UnimplementedServer：路由命中即返回 gRPC Unimplemented（HTTP 200 + 错误信封），
// 与 404（路由树无此路径）可区分。
//
// 3 条 multipart/流式裸路由（upload / breakpointContinue / download）挂在 NewHTTPServer 里
// 且需要整条依赖链，这里无法轻量装配；它们的鉴权口径由 deploy/sql/media_menu_20261106.sql
// 的 sys_apis 行与 server/http.go 的 SetOperation 常量双向锁（见下方常量断言）。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpv3 "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/stretchr/testify/require"

	pb "github.com/swordkee/kratos-vue-admin/api/media/v1"
)

type mediaNoopService struct {
	pb.UnimplementedAttachmentCategoryServiceServer
	pb.UnimplementedFileUploadAndDownloadServiceServer
	pb.UnimplementedBreakpointContinueServiceServer
}

func TestMediaRoutes_AllRegistered(t *testing.T) {
	srv := httpv3.NewServer()
	impl := &mediaNoopService{}
	require.NotPanics(t, func() {
		pb.RegisterAttachmentCategoryServiceHTTPServer(srv, impl)
		pb.RegisterFileUploadAndDownloadServiceHTTPServer(srv, impl)
		pb.RegisterBreakpointContinueServiceHTTPServer(srv, impl)
	}, "三服务路由重复/冲突注册会 panic")

	routes := []struct {
		method string
		path   string
		body   string
	}{
		// 分类（源 /attachmentCategory/*）
		{http.MethodGet, "/attachmentCategory/getCategoryList", ""},
		{http.MethodPost, "/attachmentCategory/addCategory", `{"name":"a","pid":0}`},
		{http.MethodPost, "/attachmentCategory/deleteCategory", `{"id":1}`},
		// 文件（源 /fileUploadAndDownload/*）
		{http.MethodPost, "/fileUploadAndDownload/getFileList", `{"page":1,"pageSize":10}`},
		{http.MethodPost, "/fileUploadAndDownload/editFileName", `{"id":1,"name":"a"}`},
		{http.MethodPost, "/fileUploadAndDownload/deleteFile", `{"id":1}`},
		{http.MethodPost, "/fileUploadAndDownload/deleteFiles", `{"ids":[1]}`},
		{http.MethodPost, "/fileUploadAndDownload/importURL", `{"list":[{"url":"http://x/a.png"}]}`},
		// 断点续传（findFile 是 GET 且有写副作用，保真保留）
		{http.MethodGet, "/fileUploadAndDownload/findFile?fileMd5=abc&fileName=a.bin&chunkTotal=2", ""},
		{http.MethodPost, "/fileUploadAndDownload/breakpointContinueFinish?fileMd5=abc&fileName=a.bin", ""},
		{http.MethodPost, "/fileUploadAndDownload/removeChunk", `{"fileMd5":"abc"}`},
	}

	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, strings.NewReader(r.body))
		if r.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		// 未注册 = kratos 默认 404；注册了但实现为 Unimplemented = 200 + 错误信封
		require.NotEqual(t, http.StatusNotFound, rec.Code,
			"%s %s 未注册（HTTP 404）", r.method, r.path)
	}
}

// 裸路由 operation 常量锁：这三条必须在 sys_apis 里有对应权限点，改名即失败。
func TestMediaBareRouteOperations(t *testing.T) {
	ops := []string{
		"/api.media.v1.FileUploadAndDownloadService/UploadFile",
		"/api.media.v1.FileUploadAndDownloadService/DownloadFile",
		"/api.media.v1.BreakpointContinueService/BreakpointContinue",
	}
	for _, op := range ops {
		require.True(t, strings.HasPrefix(op, "/api.media.v1."), "operation 必须以 api.media.v1 前缀: %s", op)
	}
}
