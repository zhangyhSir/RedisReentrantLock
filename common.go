package redislock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd                   // set the key exipration time
	Del(ctx context.Context, keys ...string) *redis.IntCmd                                             // delete specified key
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd // set the key value if the key does not exist
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd            // execute the Lua script
}
