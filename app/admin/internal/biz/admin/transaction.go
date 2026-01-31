package admin

import "context"

// Transaction 事务接口
type Transaction interface {
	Transaction(context.Context, func(ctx context.Context) error) error
}
