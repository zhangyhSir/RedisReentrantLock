package redislock

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultExpiration = 10 * time.Second
	script            = `
	if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
)

type RedisReentrantLock struct {
	Client     RedisClient
	mu         sync.Mutex
	Key        string
	Value      string
	locks      map[string]int
	cancelFunc context.CancelFunc
}

// create a reentrant distributed transaction lock
func NewRedisReentrantLock(client RedisClient, key string, value string) *RedisReentrantLock {
	return &RedisReentrantLock{
		Client: client,
		Key:    key,
		Value:  value,
	}
}

// attempt to acquire lock
func (rrl *RedisReentrantLock) Lock(ctx context.Context) (bool, error) {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()

	if count, ok := rrl.locks[rrl.Key]; ok {
		rrl.locks[rrl.Key] = count + 1
		return true, nil
	}

	ok, err := rrl.Client.SetNX(ctx, rrl.Key, rrl.Value, 0).Result()
	if err != nil {
		return false, err
	}

	if !ok {
		return false, errors.New("failed to acquire lock for key")
	}
	c, cancel := context.WithCancel(ctx)
	rrl.cancelFunc = cancel
	rrl.refresh(c)
	rrl.locks[rrl.Key] = 1
	return true, nil
}

// attempt to unlock
func (rrl *RedisReentrantLock) Unlock(ctx context.Context) (bool, error) {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()

	if count, ok := rrl.locks[rrl.Key]; ok {
		if count == 1 {
			delete(rrl.locks, rrl.Key)
			res, err := rrl.Client.Eval(ctx, script, []string{rrl.Key}, rrl.Value).Result()
			if err != nil {
				return false, err
			}
			if res == 0 {
				return false, errors.New("failed to release lock for key")
			}
			rrl.cancelFunc()
			return true, nil
		} else {
			rrl.locks[rrl.Key] = count - 1
		}
	}

	return true, nil
}

func (rrl *RedisReentrantLock) refresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(defaultExpiration / 6)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rrl.Client.Expire(ctx, rrl.Key, defaultExpiration)
			}
		}
	}()
}
