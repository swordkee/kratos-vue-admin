package media

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"path"
	"strings"

	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// FileUseCase 文件记录用例（源 FileUploadAndDownloadService）。
type FileUseCase struct {
	repo    FileRepo
	storage StorageRepo
	l       *log.Helper
}

// NewFileUseCase 构造文件用例。
func NewFileUseCase(repo FileRepo, storage StorageRepo, logger log.Logger) *FileUseCase {
	return &FileUseCase{repo: repo, storage: storage, l: newHelper(logger)}
}

// Upload 落存储 + 建档。
//
// noSave=="1" 时只回显记录不落库（源 UploadFile 的 noSave 语义，扫码/表单预览用）。
// 记录字段对齐源端：name=原始文件名、tag=扩展名（无点）、key=对象键、url=访问地址、
// resolution=图片宽高、size=字节、oss_type=当前存储模式、created_by=上传用户。
func (uc *FileUseCase) Upload(ctx context.Context, header *multipart.FileHeader, file multipart.File, userID int64, noSave string) (*model.ExaFileUploadAndDownloads, error) {
	if header == nil {
		return nil, ErrFileNotFound
	}
	name := path.Base(header.Filename)
	if name == "" || name == "." || name == "/" {
		return nil, ErrInvalidFileName
	}
	key := BuildKey(name)
	if err := uc.storage.Save(ctx, key, file, header.Size); err != nil {
		return nil, err
	}
	ext := strings.TrimPrefix(path.Ext(name), ".")

	rec := &model.ExaFileUploadAndDownloads{
		Name:       name,
		URL:        uc.storage.URL(key),
		Tag:        ext,
		Key:        key,
		Resolution: resolutionOf(file),
		Size:       header.Size,
		OssType:    uc.storage.Mode(),
		CreatedBy:  userID,
	}
	if noSave == "1" {
		return rec, nil
	}
	if err := uc.repo.Create(ctx, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// resolutionOf 探测图片宽高（源 upload.DecodeImageWidthHeight；解不出返回空，不报错）。
//
// 读取需要独立句柄：multipart.File 已被 Save 消费，调用方传的是重新打开的句柄。
func resolutionOf(file multipart.File) string {
	if file == nil {
		return ""
	}
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return ""
		}
	}
	return decodeResolution(file)
}

// ListPage 分页列表（源 ORDER BY updated_at DESC）。
func (uc *FileUseCase) ListPage(ctx context.Context, cond FileListCondition) ([]*model.ExaFileUploadAndDownloads, int64, error) {
	return uc.repo.ListPage(ctx, cond)
}

// Find 单条详情（下载/删除前置）。
func (uc *FileUseCase) Find(ctx context.Context, id int64) (*model.ExaFileUploadAndDownloads, error) {
	row, err := uc.repo.FindByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFileNotFound
	}
	return row, err
}

// EditName 编辑文件名/备注（源 EditFileName：仅更新 name）。
func (uc *FileUseCase) EditName(ctx context.Context, id int64, name string) error {
	if name == "" {
		return ErrInvalidFileName
	}
	if _, err := uc.Find(ctx, id); err != nil {
		return err
	}
	return uc.repo.UpdateName(ctx, id, name)
}

// Delete 删除单条：先删存储对象再删记录（源顺序一致）。
//
// 差异 2：对象已不存在（ErrObjectNotFound）不阻断删记录——导入了外链 URL 的记录
// 没有本地/桶内对象，源端会卡成「文件删除失败」永远删不掉。
func (uc *FileUseCase) Delete(ctx context.Context, id int64) error {
	row, err := uc.Find(ctx, id)
	if err != nil {
		return err
	}
	if row.Key != "" {
		if derr := uc.storage.Delete(ctx, row.Key); derr != nil && !errors.Is(derr, ErrObjectNotFound) {
			uc.l.WithContext(ctx).Warnf("删除存储对象失败 key=%s err=%v", row.Key, derr)
			return ErrFileDeleteFailed
		}
	}
	return uc.repo.DeleteHard(ctx, id)
}

// DeleteBatch 批量删除（KVA 增补）：逐条走 Delete 语义，失败项收集不中断。
func (uc *FileUseCase) DeleteBatch(ctx context.Context, ids []int64) ([]int64, error) {
	var failed []int64
	for _, id := range ids {
		if err := uc.Delete(ctx, id); err != nil {
			failed = append(failed, id)
			uc.l.WithContext(ctx).Warnf("批量删除失败 id=%d err=%v", id, err)
		}
	}
	return failed, nil
}

// ImportItem 导入 URL 的入参。
type ImportItem struct {
	Name string
	URL  string
	Tag  string
	Key  string
}

// ImportURL 批量导入外链记录（源 ImportURL：gorm Create(&slice)，只写库不下载对象）。
func (uc *FileUseCase) ImportURL(ctx context.Context, items []ImportItem) (int64, error) {
	if len(items) == 0 {
		return 0, nil
	}
	rows := make([]*model.ExaFileUploadAndDownloads, 0, len(items))
	for _, it := range items {
		tag := it.Tag
		if tag == "" {
			tag = strings.TrimPrefix(path.Ext(it.URL), ".")
		}
		rows = append(rows, &model.ExaFileUploadAndDownloads{
			Name:    it.Name,
			URL:     it.URL,
			Tag:     tag,
			Key:     it.Key,
			OssType: uc.storage.Mode(),
		})
	}
	if err := uc.repo.CreateBatch(ctx, rows); err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// Open 下载用：按 key 打开对象（本地流式；aliyun 由 service 走 302，不用本方法）。
func (uc *FileUseCase) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return uc.storage.Open(ctx, key)
}

// StorageURL 取对象访问地址（aliyun 302 直链用）。
func (uc *FileUseCase) StorageURL(key string) string { return uc.storage.URL(key) }
