package sms

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCache 把 redis.UniversalClient 适配为 Cache（Get 归一化为字符串，miss 返回空串）
type redisCache struct {
	rdb redis.UniversalClient
}

// NewRedisCache 构造缓存适配器
func NewRedisCache(rdb redis.UniversalClient) Cache {
	return &redisCache{rdb: rdb}
}

func (c *redisCache) Get(ctx context.Context, key string) string {
	v, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			// 仅日志可见的读故障按未命中处理，避免验证码流程 500
			return ""
		}
		return ""
	}
	return v
}

func (c *redisCache) Set(ctx context.Context, key, value string, expire time.Duration) error {
	return c.rdb.Set(ctx, key, value, expire).Err()
}

func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}
