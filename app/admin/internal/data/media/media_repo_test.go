// Package media_test 媒体库数据层 + 用例的联合测试（内存 SQLite，无需 MySQL）。
//
// 与 data/wallpaper 测试同套路：AutoMigrate 四张 exa_* 表 → gen Query 经
// NewStaticQuerySource 喂给仓储；存储用真实 localStorage（t.TempDir()）验证落盘与路径防护。
package media_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	bizmedia "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	datamedia "github.com/swordkee/kratos-vue-admin/app/admin/internal/data/media"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

type fixture struct {
	ctx context.Context
	q   *dao.Query
	qs  datamedia.QuerySource
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(10000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ExaAttachmentCategory{},
		&model.ExaFileUploadAndDownloads{},
		&model.ExaFiles{},
		&model.ExaFileChunks{},
	))
	q := dao.Use(db)
	return &fixture{ctx: context.Background(), q: q, qs: datamedia.NewStaticQuerySource(q)}
}

func testLogger() log.Logger {
	return log.NewHelper(log.DefaultLogger).Logger()
}

// localStorage 用真实本地实现（conf.oss.use=local + t.TempDir）。
func localStorage(t *testing.T, dir string) bizmedia.StorageRepo {
	t.Helper()
	st, err := datamedia.NewStorageRepo(&conf.Oss{
		Use:   conf.OssUseMode_local,
		Local: &conf.OssLocalConfig{Dir: dir},
	}, testLogger())
	require.NoError(t, err)
	return st
}

// ---------- 分类 ----------

func TestCategoryTreeAndDuplicate(t *testing.T) {
	f := newFixture(t)
	uc := bizmedia.NewCategoryUseCase(datamedia.NewCategoryRepo(f.qs), testLogger())

	root, err := uc.Add(f.ctx, 0, "根分类", 0)
	require.NoError(t, err)
	child, err := uc.Add(f.ctx, 0, "子分类", root.ID)
	require.NoError(t, err)

	nodes, err := uc.List(f.ctx)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "根分类", nodes[0].Name)
	require.Len(t, nodes[0].Children, 1)
	require.Equal(t, "子分类", nodes[0].Children[0].Name)

	// 同父同名 → 拒绝（真实 SQL 条件，非 mock）
	_, err = uc.Add(f.ctx, 0, "子分类", root.ID)
	require.ErrorIs(t, err, bizmedia.ErrCategoryNameDuplicated)

	// 差异 1：编辑分类不改名必须成功（源端 Take 不排除自身 ⇒ 会误报重名）
	_, err = uc.Add(f.ctx, child.ID, "子分类", root.ID)
	require.NoError(t, err)

	// 有子级拒绝删除
	require.ErrorIs(t, uc.Delete(f.ctx, root.ID), bizmedia.ErrCategoryHasChildren)
	require.NoError(t, uc.Delete(f.ctx, child.ID))

	// 硬删：Unscoped 后行已不存在（源端物理删，不留软删残行）
	_, err = f.q.ExaAttachmentCategory.WithContext(f.ctx).Unscoped().
		Where(f.q.ExaAttachmentCategory.ID.Eq(child.ID)).Take()
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ---------- 文件记录 ----------

func TestFileListPageAndDelete(t *testing.T) {
	f := newFixture(t)
	repo := datamedia.NewFileRepo(f.qs)
	uc := bizmedia.NewFileUseCase(repo, localStorage(t, t.TempDir()), testLogger())

	require.NoError(t, repo.Create(f.ctx, &model.ExaFileUploadAndDownloads{
		Name: "e2e_media_a.png", Key: "a.png", URL: "/static/a.png", Tag: "png",
	}))
	require.NoError(t, repo.Create(f.ctx, &model.ExaFileUploadAndDownloads{
		Name: "e2e_media_b.jpg", Key: "b.jpg", URL: "/static/b.jpg", Tag: "jpg",
	}))

	rows, total, err := uc.ListPage(f.ctx, bizmedia.FileListCondition{Keyword: "e2e_media", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, rows, 2)

	rows, total, err = uc.ListPage(f.ctx, bizmedia.FileListCondition{Keyword: "b.jpg", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "e2e_media_b.jpg", rows[0].Name)

	require.NoError(t, uc.EditName(f.ctx, rows[0].ID, "改名.jpg"))
	got, err := uc.Find(f.ctx, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, "改名.jpg", got.Name)

	// 删除：存储对象不存在也不阻断（差异 2）
	require.NoError(t, uc.Delete(f.ctx, rows[0].ID))
	_, err = uc.Find(f.ctx, rows[0].ID)
	require.ErrorIs(t, err, bizmedia.ErrFileNotFound)

	// 批量删除：不存在的 id 进 failed_ids，不中断
	failed, err := uc.DeleteBatch(f.ctx, []int64{999999})
	require.NoError(t, err)
	require.Equal(t, []int64{999999}, failed)

	// 导入 URL（只写库，不下载对象）
	n, err := uc.ImportURL(f.ctx, []bizmedia.ImportItem{
		{Name: "ext", URL: "https://example.com/ext.webp", Key: "e2e_import_1"},
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
}

// ---------- 断点续传 ----------

func TestChunkUploadFinishAndRemove(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	store := bizmedia.NewChunkStore(filepath.Join(root, "chunks"))
	storage := localStorage(t, root)
	fileUseCase := bizmedia.NewFileUseCase(datamedia.NewFileRepo(f.qs), storage, testLogger())
	chunkRepo := datamedia.NewChunkRepo(f.qs)
	uc := bizmedia.NewChunkUseCase(chunkRepo, datamedia.NewFileRepo(f.qs), storage, store, testLogger())

	// 20 字节，拆 2 片；首片做成真实 PNG 头以外的纯文本（分辨率应为空）
	payload := []byte("0123456789abcdefghij")
	fileMD5 := bizmedia.MD5Hex(payload)
	part1, part2 := payload[:10], payload[10:]

	row, chunks, err := uc.FindOrCreate(f.ctx, fileMD5, "e2e_chunk.bin", 2)
	require.NoError(t, err)
	require.False(t, row.IsFinish)
	require.Empty(t, chunks)

	// 分片 md5 不符 → 拒绝（源 CheckMd5）
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_chunk.bin", 0, 2, part1, bizmedia.MD5Hex([]byte("bad")))
	require.ErrorIs(t, err, bizmedia.ErrChunkMD5Mismatch)

	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_chunk.bin", 0, 2, part1, bizmedia.MD5Hex(part1))
	require.NoError(t, err)
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_chunk.bin", 1, 2, part2, bizmedia.MD5Hex(part2))
	require.NoError(t, err)

	// 断点续传回显：两个分片都在
	_, chunks, err = uc.FindOrCreate(f.ctx, fileMD5, "e2e_chunk.bin", 2)
	require.NoError(t, err)
	require.Len(t, chunks, 2)

	// 合并 + 建档（差异 3）
	res, err := uc.Finish(f.ctx, fileMD5, "e2e_chunk.bin", 7)
	require.NoError(t, err)
	require.NotEmpty(t, res.Key)
	require.EqualValues(t, len(payload), res.Size)
	require.NotZero(t, res.FileID)

	rec, err := fileUseCase.Find(f.ctx, res.FileID)
	require.NoError(t, err)
	require.Equal(t, int64(7), rec.CreatedBy)
	require.EqualValues(t, len(payload), rec.Size)
	require.Equal(t, "bin", rec.Tag)

	// 落盘内容 = 原 payload（合并顺序正确）
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.Key)))
	require.NoError(t, err)
	require.Equal(t, payload, body)

	// 再传同指纹 ⇒ 秒传（新建 is_finish 行，复用 file_path）
	row2, _, err := uc.FindOrCreate(f.ctx, fileMD5, "e2e_chunk_copy.bin", 2)
	require.NoError(t, err)
	require.True(t, row2.IsFinish)
	require.Equal(t, res.Key, row2.FilePath)

	// RemoveChunk：清分片目录 + 分片行
	require.NoError(t, uc.RemoveChunk(f.ctx, fileMD5, res.Key))
	_, statErr := os.Stat(filepath.Join(root, "chunks", fileMD5))
	require.True(t, os.IsNotExist(statErr))
	left, err := chunkRepo.ListChunks(f.ctx, row.ID)
	require.NoError(t, err)
	require.Empty(t, left)

	// 缺片 fail-closed（源端静默跳过会拼出坏文件）
	otherMD5 := bizmedia.MD5Hex([]byte("other"))
	_, _, err = uc.FindOrCreate(f.ctx, otherMD5, "e2e_missing.bin", 3)
	require.NoError(t, err)
	_, err = uc.SaveChunk(f.ctx, otherMD5, "e2e_missing.bin", 0, 3, []byte("a"), bizmedia.MD5Hex([]byte("a")))
	require.NoError(t, err)
	_, err = uc.Finish(f.ctx, otherMD5, "e2e_missing.bin", 0)
	require.ErrorIs(t, err, bizmedia.ErrChunkMissing)
}

// 审核 C3 反例锁①：分片摘要全过 ≠ 整文件正确。
// 会话申报的 file_md5 取自另一份内容，实际落盘的分片是「伪造内容」且每片 md5 都合法
// ⇒ 旧实现（只合并 + 建档）会把被替换的内容当成功入库。
func TestChunkFinishRejectsWholeMismatch(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	store := bizmedia.NewChunkStore(filepath.Join(root, "chunks"))
	storage := localStorage(t, root)
	chunkRepo := datamedia.NewChunkRepo(f.qs)
	uc := bizmedia.NewChunkUseCase(chunkRepo, datamedia.NewFileRepo(f.qs), storage, store, testLogger())

	declared := bizmedia.MD5Hex([]byte("declared-whole-content"))
	forged := []byte("forged-whole-content")
	fileName := "e2e_tampered.bin"
	row, _, err := uc.FindOrCreate(f.ctx, declared, fileName, 2)
	require.NoError(t, err)

	p1, p2 := forged[:10], forged[10:]
	_, err = uc.SaveChunk(f.ctx, declared, fileName, 0, 2, p1, bizmedia.MD5Hex(p1))
	require.NoError(t, err)
	_, err = uc.SaveChunk(f.ctx, declared, fileName, 1, 2, p2, bizmedia.MD5Hex(p2))
	require.NoError(t, err)

	_, err = uc.Finish(f.ctx, declared, fileName, 7)
	require.ErrorIs(t, err, bizmedia.ErrUploadWholeMismatch)

	// 无建档行（绝不把被污染的内容登记进媒体库）
	n, err := f.q.ExaFileUploadAndDownloads.WithContext(f.ctx).
		Where(f.q.ExaFileUploadAndDownloads.Name.Eq(fileName)).Count()
	require.NoError(t, err)
	require.Zero(t, n)

	// 会话置失败：软删（deleted_at）+ 未完成 + 无 file_path，默认作用域查不到（不可秒传/续传复用）
	got, err := f.q.ExaFiles.WithContext(f.ctx).Unscoped().Where(f.q.ExaFiles.ID.Eq(row.ID)).Take()
	require.NoError(t, err)
	require.False(t, got.IsFinish)
	require.Empty(t, got.FilePath)
	require.True(t, got.DeletedAt.Valid)
	_, err = f.q.ExaFiles.WithContext(f.ctx).Where(f.q.ExaFiles.ID.Eq(row.ID)).Take()
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// 落盘清理：分片行与暂存目录都清空
	left, err := chunkRepo.ListChunks(f.ctx, row.ID)
	require.NoError(t, err)
	require.Empty(t, left)
	_, statErr := os.Stat(filepath.Join(root, "chunks", declared))
	require.True(t, os.IsNotExist(statErr))

	// 可恢复：同一指纹重新 findFile 得到全新空会话（旧会话已作废，不会命中秒传）
	row2, chunks2, err := uc.FindOrCreate(f.ctx, declared, fileName, 2)
	require.NoError(t, err)
	require.NotEqual(t, row.ID, row2.ID)
	require.False(t, row2.IsFinish)
	require.Empty(t, chunks2)
}

// 审核 C3 反例锁②：分片摘要必填（旧实现 digest=="" 直接放行 ⇒ 分片完整性可被整体绕过）。
func TestChunkSaveChunkRequiresDigest(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	store := bizmedia.NewChunkStore(filepath.Join(root, "chunks"))
	uc := bizmedia.NewChunkUseCase(datamedia.NewChunkRepo(f.qs), datamedia.NewFileRepo(f.qs),
		localStorage(t, root), store, testLogger())

	data := []byte("chunk-0-payload")
	fileMD5 := bizmedia.MD5Hex([]byte("whole-payload"))

	// 空摘要 → 拒绝
	_, err := uc.SaveChunk(f.ctx, fileMD5, "e2e_nodigest.bin", 0, 2, data, "")
	require.ErrorIs(t, err, bizmedia.ErrChunkMD5Mismatch)
	// 非法格式（非 hex / 长度不对）→ 拒绝
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_nodigest.bin", 0, 2, data, "not-a-digest")
	require.ErrorIs(t, err, bizmedia.ErrChunkMD5Mismatch)
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_nodigest.bin", 0, 2, data, "0123456789abcdef")
	require.ErrorIs(t, err, bizmedia.ErrChunkMD5Mismatch)

	// 双口径保持：裸 md5 / sha256: 前缀都可用
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_nodigest.bin", 0, 2, data, bizmedia.MD5Hex(data))
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	_, err = uc.SaveChunk(f.ctx, fileMD5, "e2e_nodigest.bin", 1, 2, data, "sha256:"+hex.EncodeToString(sum[:]))
	require.NoError(t, err)
}

// 审核 C3 正例锁：整文件申报值也可以是 sha256 口径（浏览器 WebCrypto 无 MD5）。
func TestChunkFinishAcceptsSHA256WholeDigest(t *testing.T) {
	f := newFixture(t)
	root := t.TempDir()
	storage := localStorage(t, root)
	uc := bizmedia.NewChunkUseCase(datamedia.NewChunkRepo(f.qs), datamedia.NewFileRepo(f.qs),
		storage, bizmedia.NewChunkStore(filepath.Join(root, "chunks")), testLogger())

	payload := []byte("0123456789abcdefghij")
	sum := sha256.Sum256(payload)
	fileHash := hex.EncodeToString(sum[:]) // 裸 64 位十六进制 ⇒ 按 SHA-256 比对
	row, _, err := uc.FindOrCreate(f.ctx, fileHash, "e2e_sha256.bin", 2)
	require.NoError(t, err)
	require.Equal(t, fileHash, row.FileMd5)

	p1, p2 := payload[:10], payload[10:]
	_, err = uc.SaveChunk(f.ctx, fileHash, "e2e_sha256.bin", 0, 2, p1, bizmedia.MD5Hex(p1))
	require.NoError(t, err)
	_, err = uc.SaveChunk(f.ctx, fileHash, "e2e_sha256.bin", 1, 2, p2, bizmedia.MD5Hex(p2))
	require.NoError(t, err)

	res, err := uc.Finish(f.ctx, fileHash, "e2e_sha256.bin", 0)
	require.NoError(t, err)
	require.EqualValues(t, len(payload), res.Size)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.Key)))
	require.NoError(t, err)
	require.Equal(t, payload, body)
}

// ---------- 存储 ----------

func TestLocalStorageTraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	st := localStorage(t, dir)

	require.NoError(t, st.Save(context.Background(), "sub/a.txt", strings.NewReader("hello"), 5))
	rc, err := st.Open(context.Background(), "sub/a.txt")
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, "hello", string(got))
	require.Equal(t, "/static/sub/a.txt", st.URL("sub/a.txt"))

	_, err = st.Open(context.Background(), "../escape.txt")
	require.ErrorIs(t, err, bizmedia.ErrInvalidFileName)

	require.NoError(t, st.Delete(context.Background(), "sub/a.txt"))
	_, openErr := st.Open(context.Background(), "sub/a.txt")
	require.ErrorIs(t, openErr, bizmedia.ErrObjectNotFound)
}
