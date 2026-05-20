package otp

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements Store backed by Redis for production use.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a RedisStore with the given Redis client.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Set(ctx context.Context, key, value string, exp time.Duration) error {
	return s.client.Set(ctx, key, value, exp).Err()
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (s *RedisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStore) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (s *RedisStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

func (s *RedisStore) Expire(ctx context.Context, key string, exp time.Duration) error {
	return s.client.Expire(ctx, key, exp).Err()
}
