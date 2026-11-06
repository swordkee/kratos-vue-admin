package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	khttp "github.com/go-kratos/kratos/v3/transport/http"

	pb "github.com/swordkee/kratos-vue-admin/api/media/v1"
	bizmedia "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

// 媒体库（feat/wp-media）service 层：3 个 proto 服务 + 3 条裸路由（multipart/流式）。
//
// 迁移保真要点（源 api-admin server/api/v1/example/*）：
//   - 分类列表返回**树**（源 getChildrenList 内存组树），不是平铺；
//   - 文件列表 keyword 只模糊 name，排序 updated_at DESC；
//   - 断点续传 findFile 是 GET 且有写副作用（秒传建行 / FirstOrCreate），保真保留；
//   - 响应内层 code=0/msg=success 的形态沿用本仓壁纸域口径（外层信封由 encoder 统一）。

const mediaSuccessMsg = "success"

// currentUserInfo 取当前登录用户（authz claims）：UID + 昵称。
//
// 媒体库在**登录后**才可访问（三条 multipart/流式裸路由都带 operation，走同一套
// JWT + casbin 中间件），因此这里恒有 claims；缺失按 0 处理但不 panic（操作日志等
// 旁路调用可能没有 claims，上传落库的 created_by 退化为 0 不影响主流程）。
func currentUserInfo(ctx context.Context) (int64, string) {
	claims, err := authz.FromContext(ctx)
	if err != nil || claims == nil {
		return 0, ""
	}
	return claims.UserID, claims.Nickname
}

// ---------- 分类 ----------

// MediaCategoryService 实现 v1.AttachmentCategoryServiceHTTPServer（/attachmentCategory/*）。
type MediaCategoryService struct {
	pb.UnimplementedAttachmentCategoryServiceServer
	uc *bizmedia.CategoryUseCase
}

// NewMediaCategoryService 构造分类服务。
func NewMediaCategoryService(uc *bizmedia.CategoryUseCase) *MediaCategoryService {
	return &MediaCategoryService{uc: uc}
}

func (s *MediaCategoryService) GetCategoryList(ctx context.Context, req *pb.GetCategoryListReq) (*pb.GetCategoryListRsp, error) {
	nodes, err := s.uc.List(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetCategoryListRsp{Code: 0, Msg: mediaSuccessMsg, List: convertCategoryNodes(nodes)}, nil
}

func (s *MediaCategoryService) AddCategory(ctx context.Context, req *pb.AddCategoryReq) (*pb.AddCategoryRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	row, err := s.uc.Add(ctx, req.Id, req.Name, req.Pid)
	if err != nil {
		return nil, err
	}
	return &pb.AddCategoryRsp{Code: 0, Msg: mediaSuccessMsg, Id: row.ID}, nil
}

func (s *MediaCategoryService) DeleteCategory(ctx context.Context, req *pb.DeleteCategoryReq) (*pb.DeleteCategoryRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.uc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &pb.DeleteCategoryRsp{Code: 0, Msg: mediaSuccessMsg}, nil
}

func convertCategoryNodes(nodes []*bizmedia.CategoryNode) []*pb.MediaCategory {
	out := make([]*pb.MediaCategory, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &pb.MediaCategory{
			Id:        n.ID,
			CreatedAt: n.CreatedAt.UnixMilli(),
			UpdatedAt: n.UpdatedAt.UnixMilli(),
			Name:      n.Name,
			Pid:       int64(n.Pid),
			Children:  convertCategoryNodes(n.Children),
		})
	}
	return out
}

// ---------- 文件 ----------

// MediaFileService 实现 v1.FileUploadAndDownloadServiceHTTPServer（/fileUploadAndDownload/*）。
type MediaFileService struct {
	pb.UnimplementedFileUploadAndDownloadServiceServer
	uc *bizmedia.FileUseCase
}

// NewMediaFileService 构造文件服务。
func NewMediaFileService(uc *bizmedia.FileUseCase) *MediaFileService {
	return &MediaFileService{uc: uc}
}

func (s *MediaFileService) GetFileList(ctx context.Context, req *pb.GetFileListReq) (*pb.GetFileListRsp, error) {
	rows, total, err := s.uc.ListPage(ctx, bizmedia.FileListCondition{
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	list := make([]*pb.MediaFile, 0, len(rows))
	for _, r := range rows {
		list = append(list, convertMediaFile(r))
	}
	return &pb.GetFileListRsp{
		Code: 0, Msg: mediaSuccessMsg, Total: total,
		Page: req.Page, PageSize: req.PageSize, List: list,
	}, nil
}

func (s *MediaFileService) EditFileName(ctx context.Context, req *pb.EditFileNameReq) (*pb.EditFileNameRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.uc.EditName(ctx, req.Id, req.Name); err != nil {
		return nil, err
	}
	return &pb.EditFileNameRsp{Code: 0, Msg: mediaSuccessMsg}, nil
}

func (s *MediaFileService) DeleteFile(ctx context.Context, req *pb.DeleteFileReq) (*pb.DeleteFileRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.uc.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &pb.DeleteFileRsp{Code: 0, Msg: mediaSuccessMsg}, nil
}

func (s *MediaFileService) DeleteFiles(ctx context.Context, req *pb.DeleteFilesReq) (*pb.DeleteFilesRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	failed, err := s.uc.DeleteBatch(ctx, req.Ids)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteFilesRsp{Code: 0, Msg: mediaSuccessMsg, FailedIds: failed}, nil
}

func (s *MediaFileService) ImportURL(ctx context.Context, req *pb.ImportURLReq) (*pb.ImportURLRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	items := make([]bizmedia.ImportItem, 0, len(req.List))
	for _, it := range req.List {
		items = append(items, bizmedia.ImportItem{Name: it.Name, URL: it.Url, Tag: it.Tag, Key: it.Key})
	}
	n, err := s.uc.ImportURL(ctx, items)
	if err != nil {
		return nil, err
	}
	return &pb.ImportURLRsp{Code: 0, Msg: mediaSuccessMsg, Count: n}, nil
}

// RESTUploadFile 普通上传裸路由（POST /fileUploadAndDownload/upload，multipart 字段 file）。
//
// query/form 参数 noSave=1 时只回显不落库（源 UploadFile 的 noSave 语义）。
// 返回值沿用源形态：{file:{...}}（data 层包一层 file，前端读取 data.file）。
func (s *MediaFileService) RESTUploadFile(ctx khttp.Context) (map[string]any, error) {
	file, header, err := ctx.Request().FormFile("file")
	if err != nil {
		return nil, kerrors.BadRequest("MEDIA_UPLOAD_INVALID", "接收文件失败")
	}
	defer file.Close()
	noSave := ctx.Request().URL.Query().Get("noSave")
	if noSave == "" {
		// 源端从 query 取 noSave；multipart 表单里给同名字段也接受（kratos Form() 是 []string）。
		if vals := ctx.Form()["noSave"]; len(vals) > 0 {
			noSave = vals[0]
		}
	}
	if noSave == "" {
		noSave = "0"
	}
	userID, _ := currentUserInfo(ctx)
	rec, err := s.uc.Upload(ctx, header, file, userID, noSave)
	if err != nil {
		return nil, err
	}
	return map[string]any{"file": convertMediaFile(rec)}, nil
}

// RESTDownload 下载裸路由（GET /fileUploadAndDownload/download?id=|key=）。
//
// 本地存储：流式回包 + Content-Disposition: attachment；
// aliyun：302 到对象直链（签名域名由配置决定，服务端不做中转）。
func (s *MediaFileService) RESTDownload(ctx khttp.Context) error {
	q := ctx.Request().URL.Query()
	idStr := q.Get("id")
	key := q.Get("key")
	var rec *model.ExaFileUploadAndDownloads
	if idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			return kerrors.BadRequest("MEDIA_PARAM_INVALID", "id 不合法")
		}
		rec, err = s.uc.Find(ctx, id)
		if err != nil {
			return err
		}
		key = rec.Key
	}
	if key == "" {
		return kerrors.BadRequest("MEDIA_PARAM_INVALID", "缺少 id 或 key")
	}
	name := key
	if rec != nil && rec.Name != "" {
		name = rec.Name
	}
	if u := s.uc.StorageURL(key); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		ctx.Response().Header().Set("Location", u)
		ctx.Response().WriteHeader(302)
		return nil
	}
	rc, err := s.uc.Open(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()
	h := ctx.Response().Header()
	h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	h.Set("Content-Type", "application/octet-stream")
	ctx.Response().WriteHeader(200)
	_, err = io.Copy(ctx.Response(), rc)
	return err
}

func convertMediaFile(f *model.ExaFileUploadAndDownloads) *pb.MediaFile {
	if f == nil {
		return nil
	}
	return &pb.MediaFile{
		Id:         f.ID,
		CreatedAt:  f.CreatedAt.UnixMilli(),
		UpdatedAt:  f.UpdatedAt.UnixMilli(),
		Name:       f.Name,
		Url:        f.URL,
		Tag:        f.Tag,
		Key:        f.Key,
		Resolution: f.Resolution,
		Size:       f.Size,
		OssType:    f.OssType,
		CreatedBy:  f.CreatedBy,
	}
}

// ---------- 断点续传 ----------

// MediaBreakpointService 实现 v1.BreakpointContinueServiceHTTPServer。
type MediaBreakpointService struct {
	pb.UnimplementedBreakpointContinueServiceServer
	uc *bizmedia.ChunkUseCase
}

// NewMediaBreakpointService 构造断点续传服务。
func NewMediaBreakpointService(uc *bizmedia.ChunkUseCase) *MediaBreakpointService {
	return &MediaBreakpointService{uc: uc}
}

func (s *MediaBreakpointService) FindFile(ctx context.Context, req *pb.FindFileReq) (*pb.FindFileRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	row, chunks, err := s.uc.FindOrCreate(ctx, req.FileMd5, req.FileName, req.ChunkTotal)
	if err != nil {
		return nil, err
	}
	file := &pb.ExaFile{
		Id:         row.ID,
		FileName:   row.FileName,
		FileMd5:    row.FileMd5,
		FilePath:   row.FilePath,
		ChunkTotal: row.ChunkTotal,
		IsFinish:   row.IsFinish,
	}
	for _, c := range chunks {
		file.Chunks = append(file.Chunks, &pb.FileChunk{
			Id:              c.ID,
			ExaFileId:       c.ExaFileID,
			FileChunkNumber: c.FileChunkNumber,
			FileChunkPath:   c.FileChunkPath,
		})
	}
	return &pb.FindFileRsp{Code: 0, Msg: mediaSuccessMsg, File: file}, nil
}

// normalizeBreakpointError 收敛断点续传的业务哨兵：整文件校验失败必须是 4xx 中文文案
// （错误不回显红线：kratos 默认 encoder 会把非 kerrors 的错误按 500 + err.Error() 透出内部细节）。
func normalizeBreakpointError(err error) error {
	if errors.Is(err, bizmedia.ErrUploadWholeMismatch) {
		return kerrors.BadRequest("MEDIA_WHOLE_MISMATCH", "文件校验未通过，请重新上传")
	}
	return err
}

func (s *MediaBreakpointService) BreakpointContinueFinish(ctx context.Context, req *pb.BreakpointContinueFinishReq) (*pb.BreakpointContinueFinishRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	userID, _ := currentUserInfo(ctx)
	res, err := s.uc.Finish(ctx, req.FileMd5, req.FileName, userID)
	if err != nil {
		return nil, normalizeBreakpointError(err)
	}
	return &pb.BreakpointContinueFinishRsp{
		Code: 0, Msg: mediaSuccessMsg,
		FilePath: res.Key, Url: res.URL, Size: res.Size,
		FileId: strconv.FormatInt(res.FileID, 10),
	}, nil
}

func (s *MediaBreakpointService) RemoveChunk(ctx context.Context, req *pb.RemoveChunkReq) (*pb.RemoveChunkRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if err := s.uc.RemoveChunk(ctx, req.FileMd5, req.FilePath); err != nil {
		return nil, err
	}
	return &pb.RemoveChunkRsp{Code: 0, Msg: mediaSuccessMsg}, nil
}

// RESTBreakpointContinue 分片上传裸路由（POST /fileUploadAndDownload/breakpointContinue）。
//
// form 字段与源端一致：file / fileMd5 / fileName / chunkNumber / chunkTotal / chunkMd5。
func (s *MediaBreakpointService) RESTBreakpointContinue(ctx khttp.Context) error {
	form := ctx.Request()
	if err := form.ParseMultipartForm(32 << 20); err != nil {
		return kerrors.BadRequest("MEDIA_PARAM_INVALID", "接收文件失败")
	}
	f, _, err := form.FormFile("file")
	if err != nil {
		return kerrors.BadRequest("MEDIA_PARAM_INVALID", "接收文件失败")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return kerrors.InternalServer("MEDIA_READ_FAILED", "文件读取失败")
	}
	fileMD5 := form.FormValue("fileMd5")
	fileName := form.FormValue("fileName")
	chunkMD5 := form.FormValue("chunkMd5")
	number, _ := strconv.Atoi(form.FormValue("chunkNumber"))
	total, _ := strconv.Atoi(form.FormValue("chunkTotal"))
	if _, err := s.uc.SaveChunk(ctx, fileMD5, fileName, int32(number), int32(total), data, chunkMD5); err != nil {
		return err
	}
	return nil
}
