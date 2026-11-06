// Package media 媒体库数据层：分类/文件/断点续传三组仓储 + 存储与分片暂存盘装配。
//
// 仓储统一从 QuerySource 取 gen Query（*data.Data 实现，事务态 Query 经 ctx 透传）；
// 需要 gen 表达不了的条件（LIKE / 排序 / 排除自身）直接用底层 *gorm.DB。
package media

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
)

// QuerySource 抽取 *data.Data 的最小依赖：按 ctx 取（可能带事务的）gen Query。
type QuerySource interface {
	Query(ctx context.Context) *dao.Query
}

// pageLimitOffset 与 data.convertPageSize 同口径（page 从 1 起，缺省 size=10）。
func pageLimitOffset(page, size int32) (limit, offset int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	return int(size), int((page - 1) * size)
}

// staticQuerySource 直接持有 *dao.Query 的 QuerySource 实现（仓储测试用）。
type staticQuerySource struct {
	q *dao.Query
}

// NewStaticQuerySource 用固定 dao.Query 构造 QuerySource（测试用）。
func NewStaticQuerySource(q *dao.Query) QuerySource {
	return &staticQuerySource{q: q}
}

func (s *staticQuerySource) Query(_ context.Context) *dao.Query { return s.q }
