// Package media 媒体库（附件/上传）业务域：分类树、文件记录、断点续传。
//
// 语义来源：api-admin（gin-vue-admin）server/{api,service}/example：
//   - exa_attachment_category.go     分类（getChildrenList 内存组树、同名同父去重、删前查子级、Unscoped 硬删）
//   - exa_file_upload_download.go    文件记录（Upload/EditFileName/DeleteFile/GetFileRecordInfoList/ImportURL）
//   - exa_breakpoint_continue.go     断点续传（FindOrCreateFile/CreateFileChunk/DeleteFileChunk）
//
// 分层形态照本仓 biz/{admin,wallpaper,distribute}：repo 接口在此定义、薄 usecase 只做业务判定，
// 存储（local/aliyun）与分片暂存盘分别由 StorageRepo / ChunkStore 抽象，均注入不依赖全局单例。
//
// 与源端的**有意差异**（均在本包内注释标明并配 e2e 正向锁）：
//  1. 分类同名同父去重的判定排除自身（源 Take 不排除 ⇒ 编辑分类不改名会误报「分类名称已存在」）；
//  2. 文件删除：存储对象已不存在时不再报错（源 oss.DeleteFile 直接失败 ⇒ 导入 URL 建档的行永远删不掉）；
//  3. 断点续传合并完成后建档（源仅合并落盘、不写文件记录 ⇒ 分片上传的文件在媒体库里看不到）；
//  4. 批量删除为 KVA 增补（源端只有单选删除）；
//  5. 合并后校验**整文件**摘要（源端只验分片、且空摘要直接放行，见 chunkDigestMatch）⇒
//     合并结果与申报指纹不符即作废会话（审核 C3，对齐 kv UploadUseCase.Complete）。
package media

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// 业务哨兵错误（service 层据此给中文文案，不依赖具体 DB 错误文本）。
var (
	ErrCategoryNameDuplicated = errors.New("分类名称已存在")
	ErrCategoryHasChildren    = errors.New("请先删除子级")
	ErrFileNotFound           = errors.New("文件不存在")
	ErrFileDeleteFailed       = errors.New("文件删除失败")
	ErrChunkMD5Mismatch       = errors.New("检查md5失败")
	ErrUploadWholeMismatch    = errors.New("整文件 MD5 不匹配")
	ErrChunkMissing           = errors.New("分片缺失")
	ErrInvalidFileName        = errors.New("文件名不合法")
	ErrObjectNotFound         = errors.New("存储对象不存在")
)

// CategoryRepo 分类仓储（exa_attachment_category）。
type CategoryRepo interface {
	FindAll(ctx context.Context) ([]*model.ExaAttachmentCategory, error)
	// FindByNameAndPid 同父同名查重（excludeID>0 时排除自身）。
	FindByNameAndPid(ctx context.Context, name string, pid, excludeID int64) (*model.ExaAttachmentCategory, error)
	Create(ctx context.Context, c *model.ExaAttachmentCategory) error
	UpdateNamePid(ctx context.Context, id int64, name string, pid int64) error
	CountChildren(ctx context.Context, pid int64) (int64, error)
	DeleteHard(ctx context.Context, id int64) error
}

// CategoryNode 分类树节点（gen 模型无自关联字段，树在 biz 组装）。
type CategoryNode struct {
	*model.ExaAttachmentCategory
	Children []*CategoryNode
}

// FileListCondition 文件列表条件（对齐源 GetFileRecordInfoList：仅 keyword 模糊 name）。
type FileListCondition struct {
	Keyword  string
	Page     int32
	PageSize int32
}

// FileRepo 文件记录仓储（exa_file_upload_and_downloads）。
type FileRepo interface {
	Create(ctx context.Context, f *model.ExaFileUploadAndDownloads) error
	CreateBatch(ctx context.Context, files []*model.ExaFileUploadAndDownloads) error
	FindByID(ctx context.Context, id int64) (*model.ExaFileUploadAndDownloads, error)
	CountByKey(ctx context.Context, key string) (int64, error)
	UpdateName(ctx context.Context, id int64, name string) error
	DeleteHard(ctx context.Context, id int64) error
	ListPage(ctx context.Context, cond FileListCondition) ([]*model.ExaFileUploadAndDownloads, int64, error)
}

// ChunkRepo 断点续传仓储（exa_files + exa_file_chunks）。
type ChunkRepo interface {
	// FindFinishedByMD5 查已完成（is_finish=1）的同指纹记录（秒传判定）。
	FindFinishedByMD5(ctx context.Context, fileMD5 string) (*model.ExaFiles, error)
	FindByMD5AndName(ctx context.Context, fileMD5, fileName string) (*model.ExaFiles, error)
	CreateFile(ctx context.Context, f *model.ExaFiles) error
	// UpdateFinished 置 is_finish=1 并写 file_path。
	UpdateFinished(ctx context.Context, id int64, filePath string) error
	// MarkFailed 作废被污染的会话（wp 的 exa_files 无 status 列 ⇒ 软删标记失败，
	// 默认作用域下该会话既不能被秒传命中也不能被续传复用）。
	MarkFailed(ctx context.Context, id int64) error
	ListChunks(ctx context.Context, fileID int64) ([]*model.ExaFileChunks, error)
	CreateChunk(ctx context.Context, c *model.ExaFileChunks) error
	DeleteChunks(ctx context.Context, fileID int64) error
}

// StorageRepo 对象存储抽象（local / aliyun 两实现，见 data/media/storage_*.go）。
//
// key 为对象键（不含前缀），url 由实现拼出可访问地址：local 走 /static/ 静态前缀，
// aliyun 走 imgDomain CDN 直链。
type StorageRepo interface {
	Mode() string
	URL(key string) string
	Save(ctx context.Context, key string, r io.Reader, size int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// chunkStoreUsecase 共用：logger 字段名在本包内统一 l。
func newHelper(logger log.Logger) *log.Helper {
	return log.NewHelper(log.With(logger, "module", "app/admin/internal/biz/media"))
}

// md5Hex 计算 MD5 十六进制串（对象键命名与分片校验共用）。
func md5Hex(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

// ValidateKey 校验对象键（禁止路径穿越；与源 utils.sanitizeFileMd5 同级强度）。
func ValidateKey(key string) error {
	if key == "" {
		return ErrInvalidFileName
	}
	clean := path.Clean(key)
	if clean != key || strings.HasPrefix(clean, "..") || strings.Contains(clean, "..") ||
		strings.ContainsAny(key, `\:`) {
		return ErrInvalidFileName
	}
	return nil
}

// BuildKey 生成对象键：md5(文件名去扩展名)_20060102150405.ext（对齐源 utils/upload/local.go 命名）。
func BuildKey(fileName string) string {
	ext := path.Ext(fileName)
	base := strings.TrimSuffix(path.Base(fileName), ext)
	sum := md5Hex([]byte(base))
	return sum + "_" + time.Now().Format("20060102150405") + ext
}
