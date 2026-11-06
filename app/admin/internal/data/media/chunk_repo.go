package media

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// ChunkRepo 实现 biz/media.ChunkRepo（exa_files + exa_file_chunks）。
type ChunkRepo struct {
	qs QuerySource
}

var _ media.ChunkRepo = (*ChunkRepo)(nil)

// NewChunkRepo 构造断点续传仓储。
func NewChunkRepo(qs QuerySource) *ChunkRepo {
	return &ChunkRepo{qs: qs}
}

// FindFinishedByMD5 已完成记录（秒传判定）。
func (r *ChunkRepo) FindFinishedByMD5(ctx context.Context, fileMD5 string) (*model.ExaFiles, error) {
	q := r.qs.Query(ctx).ExaFiles
	return q.WithContext(ctx).Where(q.FileMd5.Eq(fileMD5), q.IsFinish.Is(true)).First()
}

func (r *ChunkRepo) FindByMD5AndName(ctx context.Context, fileMD5, fileName string) (*model.ExaFiles, error) {
	q := r.qs.Query(ctx).ExaFiles
	return q.WithContext(ctx).Where(q.FileMd5.Eq(fileMD5), q.FileName.Eq(fileName)).First()
}

func (r *ChunkRepo) CreateFile(ctx context.Context, f *model.ExaFiles) error {
	return r.qs.Query(ctx).ExaFiles.WithContext(ctx).Create(f)
}

// UpdateFinished 置 is_finish=1 并写 file_path（源 Updates(map{"IsFinish":true,"file_path":path})）。
func (r *ChunkRepo) UpdateFinished(ctx context.Context, id int64, filePath string) error {
	q := r.qs.Query(ctx).ExaFiles
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).
		UpdateColumnSimple(q.IsFinish.Value(true), q.FilePath.Value(filePath))
	return err
}

// MarkFailed 作废被污染的会话：wp 的 exa_files 没有 status 列（kv 有），失败态用软删（deleted_at）
// 表示——保留审计痕迹，且 gen 默认作用域会把它排除在秒传/续传查询之外（fail-closed）。
func (r *ChunkRepo) MarkFailed(ctx context.Context, id int64) error {
	q := r.qs.Query(ctx).ExaFiles
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

func (r *ChunkRepo) ListChunks(ctx context.Context, fileID int64) ([]*model.ExaFileChunks, error) {
	q := r.qs.Query(ctx).ExaFileChunks
	return q.WithContext(ctx).Where(q.ExaFileID.Eq(fileID)).Order(q.FileChunkNumber).Find()
}

func (r *ChunkRepo) CreateChunk(ctx context.Context, c *model.ExaFileChunks) error {
	return r.qs.Query(ctx).ExaFileChunks.WithContext(ctx).Create(c)
}

// DeleteChunks 物理删分片行（源 Unscoped().Delete）。
func (r *ChunkRepo) DeleteChunks(ctx context.Context, fileID int64) error {
	q := r.qs.Query(ctx).ExaFileChunks
	_, err := q.WithContext(ctx).Unscoped().Where(q.ExaFileID.Eq(fileID)).Delete()
	return err
}
