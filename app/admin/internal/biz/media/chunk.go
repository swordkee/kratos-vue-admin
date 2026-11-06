package media

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// 分片暂存盘布局（对照源 utils/breakpoint_continue.go 的 ./breakpointDir/<md5>/<name>_<n>）：
//
//	<root>/<fileMD5>/<n>.part        分片
//	<root>/<fileMD5>/...             合并完成后整体删除
//
// 与源的差异：分片文件名只用序号（源把原始文件名拼进路径，需额外过滤 ".."/分隔符）。
var safeMD5 = regexp.MustCompile(`^[0-9a-zA-Z_-]{1,128}$`)

// ChunkStore 分片暂存盘（纯文件系统，无 DB；root 由 data 层按 conf 注入）。
type ChunkStore struct {
	root string
}

// NewChunkStore 构造分片暂存盘。
func NewChunkStore(root string) *ChunkStore {
	if root == "" {
		root = filepath.Join("uploads", "chunks")
	}
	return &ChunkStore{root: root}
}

// Root 暂存根目录（便于运维排查）。
func (s *ChunkStore) Root() string { return s.root }

func (s *ChunkStore) dir(fileMD5 string) (string, error) {
	if !safeMD5.MatchString(fileMD5) {
		return "", fmt.Errorf("%w: 非法的 fileMd5", ErrInvalidFileName)
	}
	return filepath.Join(s.root, fileMD5), nil
}

// SaveChunk 写入一个分片（覆盖写 ⇒ 重传幂等），返回落盘路径。
func (s *ChunkStore) SaveChunk(fileMD5 string, number int32, data []byte) (string, error) {
	if number < 0 {
		return "", fmt.Errorf("%w: 分片序号非法", ErrInvalidFileName)
	}
	dir, err := s.dir(fileMD5)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, strconv.Itoa(int(number))+".part")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// Received 扫描已落盘分片序号（断点续传回显用）。
func (s *ChunkStore) Received(fileMD5 string) []int32 {
	dir, err := s.dir(fileMD5)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []int32
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".part")
		if n, err := strconv.Atoi(name); err == nil {
			out = append(out, int32(n))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Remove 删除该文件的全部分片（源 utils.RemoveChunk）。
func (s *ChunkStore) Remove(fileMD5 string) error {
	dir, err := s.dir(fileMD5)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Merge 按 0..total-1 顺序合并到临时文件，返回临时文件路径与总字节数。
//
// 缺片即失败（源 MakeFile 会静默跳过缺失分片、拼出坏文件——本仓改为 fail-closed）。
func (s *ChunkStore) Merge(fileMD5 string, total int32) (tmpPath string, size int64, err error) {
	if total <= 0 {
		return "", 0, fmt.Errorf("%w: chunkTotal 非法", ErrInvalidFileName)
	}
	dir, err := s.dir(fileMD5)
	if err != nil {
		return "", 0, err
	}
	tmp, err := os.CreateTemp("", "kva-media-merge-*")
	if err != nil {
		return "", 0, err
	}
	tmpPath = tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	for i := int32(0); i < total; i++ {
		p := filepath.Join(dir, strconv.Itoa(int(i))+".part")
		in, oerr := os.Open(p)
		if oerr != nil {
			cleanup()
			return "", 0, fmt.Errorf("%w: %d", ErrChunkMissing, i)
		}
		n, cerr := io.Copy(tmp, in)
		_ = in.Close()
		if cerr != nil {
			cleanup()
			return "", 0, cerr
		}
		size += n
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", 0, err
	}
	return tmpPath, size, nil
}

// ChunkUseCase 断点续传用例（源 FileUploadAndDownloadService 的
// FindOrCreateFile/CreateFileChunk/DeleteFileChunk + utils.{BreakPointContinue,MakeFile,RemoveChunk}）。
type ChunkUseCase struct {
	repo    ChunkRepo
	files   FileRepo
	storage StorageRepo
	chunks  *ChunkStore
	l       *log.Helper
}

// NewChunkUseCase 构造断点续传用例。
func NewChunkUseCase(repo ChunkRepo, files FileRepo, storage StorageRepo, chunks *ChunkStore, logger log.Logger) *ChunkUseCase {
	return &ChunkUseCase{repo: repo, files: files, storage: storage, chunks: chunks, l: newHelper(logger)}
}

// FindOrCreate 对齐源 FindOrCreateFile：
//   - 同指纹且已有完成记录 ⇒ 秒传：新建一条 is_finish=1 记录并复用 file_path；
//   - 否则按 file_md5+file_name 取（无则建），回带已落盘分片。
func (uc *ChunkUseCase) FindOrCreate(ctx context.Context, fileMD5, fileName string, chunkTotal int32) (*model.ExaFiles, []*model.ExaFileChunks, error) {
	if fileMD5 == "" || fileName == "" {
		return nil, nil, ErrInvalidFileName
	}
	if fin, err := uc.repo.FindFinishedByMD5(ctx, fileMD5); err == nil && fin.FilePath != "" {
		row := &model.ExaFiles{
			FileName:   fileName,
			FileMd5:    fileMD5,
			FilePath:   fin.FilePath,
			ChunkTotal: chunkTotal,
			IsFinish:   true,
		}
		if err := uc.repo.CreateFile(ctx, row); err != nil {
			return nil, nil, err
		}
		return row, nil, nil
	}
	row, err := uc.repo.FindByMD5AndName(ctx, fileMD5, fileName)
	switch {
	case err == nil:
		if row.ChunkTotal != chunkTotal && chunkTotal > 0 {
			// 源端 FirstOrCreate 不会更新 chunkTotal；这里以本次请求为准，避免旧会话残留导致合并缺片。
			row.ChunkTotal = chunkTotal
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		row = &model.ExaFiles{FileName: fileName, FileMd5: fileMD5, ChunkTotal: chunkTotal}
		if err := uc.repo.CreateFile(ctx, row); err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, err
	}
	chunks, err := uc.repo.ListChunks(ctx, row.ID)
	if err != nil {
		return row, nil, err
	}
	return row, chunks, nil
}

// SaveChunk 校验分片摘要（源 CheckMd5）→ 落盘 → 记分片行。
//
// chunkMD5 支持两种格式（与前端能力对齐）：
//   - 裸 32 位十六进制 ⇒ 按 MD5 比较（源端 / e2e 口径）；
//   - "sha256:<hex>" ⇒ 按 SHA-256 比较（浏览器 WebCrypto 只能算 SHA-x，无 MD5）。
//
// 摘要必填：留空或格式非法一律拒绝（对齐 kv SaveChunk —— 源端 CheckMd5 必填，
// 旧实现在留空时直接放行，等于分片完整性可被整体绕过，见审核 C3）。
func (uc *ChunkUseCase) SaveChunk(ctx context.Context, fileMD5, fileName string, number, total int32, data []byte, chunkMD5 string) (*model.ExaFileChunks, error) {
	if !chunkDigestMatch(data, chunkMD5) {
		return nil, ErrChunkMD5Mismatch
	}
	row, _, err := uc.FindOrCreate(ctx, fileMD5, fileName, total)
	if err != nil {
		return nil, err
	}
	p, err := uc.chunks.SaveChunk(fileMD5, number, data)
	if err != nil {
		return nil, err
	}
	chunk := &model.ExaFileChunks{ExaFileID: row.ID, FileChunkNumber: number, FileChunkPath: p}
	if err := uc.repo.CreateChunk(ctx, chunk); err != nil {
		return nil, err
	}
	return chunk, nil
}

// FinishResult 合并结果。
type FinishResult struct {
	Key    string
	URL    string
	Size   int64
	FileID int64
}

// Finish 合并分片 → **校验整文件摘要** → 落对象存储 → 置完成 → 建档（差异 3：源只合并落盘，不建档）。
//
// 整文件校验（差异 5 / 审核 C3）：分片摘要只能证明「每一片没被改」，合并顺序错、
// 分片被替换后重传、会话指纹与内容不符这几种情况逐片校验都过。因此合并完成后按会话
// 申报的 file_md5 复算整文件摘要，不符即删合并产物 + 作废会话 + 清分片/暂存目录
// （对齐 kv UploadUseCase.Complete 的 ErrUploadWholeMismatch）。
func (uc *ChunkUseCase) Finish(ctx context.Context, fileMD5, fileName string, userID int64) (*FinishResult, error) {
	row, err := uc.repo.FindByMD5AndName(ctx, fileMD5, fileName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	if row.IsFinish && row.FilePath != "" {
		// 秒传：对象已在，直接按 key 返回（不再重复合并）。
		return &FinishResult{Key: row.FilePath, URL: uc.storage.URL(row.FilePath), Size: 0}, nil
	}
	if row.ChunkTotal <= 0 {
		return nil, fmt.Errorf("%w: chunkTotal 缺失", ErrInvalidFileName)
	}
	tmpPath, size, err := uc.chunks.Merge(fileMD5, row.ChunkTotal)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	// 合并产物必须与申报指纹一致；不一致说明内容被替换/拼错 ⇒ 会话作废，绝不建档。
	matched, err := wholeFileDigestMatch(tmpPath, row.FileMd5)
	if err != nil {
		return nil, err
	}
	if !matched {
		uc.discardUpload(ctx, row.ID, fileMD5)
		return nil, fmt.Errorf("%w: 合并结果与会话申报值不一致", ErrUploadWholeMismatch)
	}

	tmp, err := os.Open(tmpPath)
	if err != nil {
		return nil, err
	}
	defer tmp.Close()
	resolution := decodeResolution(tmp)
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	key := BuildKey(fileName)
	if err := uc.storage.Save(ctx, key, tmp, size); err != nil {
		return nil, err
	}
	if err := uc.repo.UpdateFinished(ctx, row.ID, key); err != nil {
		return nil, err
	}
	rec := &model.ExaFileUploadAndDownloads{
		Name:       fileName,
		URL:        uc.storage.URL(key),
		Tag:        strings.TrimPrefix(filepath.Ext(fileName), "."),
		Key:        key,
		Resolution: resolution,
		Size:       size,
		OssType:    uc.storage.Mode(),
		CreatedBy:  userID,
	}
	if err := uc.files.Create(ctx, rec); err != nil {
		return nil, err
	}
	return &FinishResult{Key: key, URL: rec.URL, Size: size, FileID: rec.ID}, nil
}

// RemoveChunk 清分片（源 RemoveChunk：置完成 + 写 file_path + 删分片行，另删暂存目录）。
func (uc *ChunkUseCase) RemoveChunk(ctx context.Context, fileMD5, filePath string) error {
	if err := uc.chunks.Remove(fileMD5); err != nil {
		return err
	}
	row, err := uc.repo.FindFinishedByMD5(ctx, fileMD5)
	if err != nil {
		// 没有完成记录（异常调用）时只清盘不报错，保持幂等。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if filePath != "" && row.FilePath != filePath {
		if err := uc.repo.UpdateFinished(ctx, row.ID, filePath); err != nil {
			return err
		}
	}
	return uc.repo.DeleteChunks(ctx, row.ID)
}

// chunkDigestMatch 分片摘要比对（md5 裸串 / sha256: 前缀两种形态）。
//
// 空摘要或非法摘要一律不匹配（fail-closed）：摘要必填是分片完整性的唯一证据。
func chunkDigestMatch(data []byte, digest string) bool {
	alg, want, ok := parseDigest(digest)
	if !ok {
		return false
	}
	if alg == digestSHA256 {
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]) == want
	}
	return md5Hex(data) == want
}

// 摘要算法标识。
const (
	digestMD5    = "md5"
	digestSHA256 = "sha256"
)

// parseDigest 解析摘要声明值 → (算法, 规范化小写 hex, 是否合法)。
//
// 双口径（分片与整文件共用）：
//   - "md5:" 前缀或 32 位十六进制 ⇒ MD5；
//   - "sha256:" 前缀或 64 位十六进制 ⇒ SHA-256（浏览器 WebCrypto 只能算 SHA-x）。
//
// 其余（空串、长度或字符不合法）→ ok=false，调用方必须按不匹配处理。
func parseDigest(declared string) (alg, want string, ok bool) {
	d := strings.ToLower(strings.TrimSpace(declared))
	if h, found := strings.CutPrefix(d, "sha256:"); found {
		if !isHexLen(h, 64) {
			return "", "", false
		}
		return digestSHA256, h, true
	}
	if h, found := strings.CutPrefix(d, "md5:"); found {
		if !isHexLen(h, 32) {
			return "", "", false
		}
		return digestMD5, h, true
	}
	switch len(d) {
	case 64:
		if !isHexLen(d, 64) {
			return "", "", false
		}
		return digestSHA256, d, true
	case 32:
		if !isHexLen(d, 32) {
			return "", "", false
		}
		return digestMD5, d, true
	}
	return "", "", false
}

// isHexLen 判定 s 是否恰好 n 位十六进制。
func isHexLen(s string, n int) bool {
	if len(s) != n {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// wholeFileDigestMatch 复算整文件摘要并与会话申报值比对。
func wholeFileDigestMatch(path, declared string) (bool, error) {
	alg, want, ok := parseDigest(declared)
	if !ok {
		// 申报值不可解析 ⇒ 无法证明完整性，按不符处理（文案由上层给，不回显原文）。
		return false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var h hash.Hash
	if alg == digestSHA256 {
		h = sha256.New()
	} else {
		h = md5.New()
	}
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == want, nil
}

// discardUpload 作废被污染的会话（整文件校验失败时调用）。
//
// wp 的 exa_files 只有 is_finish（没有 kv 的 status 列），失败态用**软删**表达：
// 保留审计痕迹（deleted_at），且默认作用域下该会话既不会被秒传命中、也不能被续传复用
// —— 前端重新 findFile 会拿到全新会话，必须整体重传。
//
// 清理动作：清分片行 + 删暂存目录；临时合并产物由 Finish 的 defer 删除，不进对象存储。
func (uc *ChunkUseCase) discardUpload(ctx context.Context, fileID int64, fileMD5 string) {
	if err := uc.repo.MarkFailed(ctx, fileID); err != nil {
		uc.l.WithContext(ctx).Errorf("整文件校验失败后作废会话出错: id=%d err=%v", fileID, err)
	}
	if err := uc.repo.DeleteChunks(ctx, fileID); err != nil {
		uc.l.WithContext(ctx).Errorf("整文件校验失败后清分片行出错: id=%d err=%v", fileID, err)
	}
	if err := uc.chunks.Remove(fileMD5); err != nil {
		uc.l.WithContext(ctx).Errorf("整文件校验失败后清暂存目录出错: md5=%s err=%v", fileMD5, err)
	}
}

// MD5Hex 对外暴露分片校验用（e2e/调用方自算分片 md5）。
func MD5Hex(b []byte) string { return md5Hex(b) }

// HexLower 规范化十六进制串（便于比较）。
func HexLower(s string) string {
	if b, err := hex.DecodeString(s); err == nil {
		return hex.EncodeToString(b)
	}
	return strings.ToLower(s)
}
