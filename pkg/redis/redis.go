package redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

var Rdb *redis.Client

func NewRedis() error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "10.0.0.105:6379",
		Password: "", // 密码
		DB:       0,  // 数据库
		PoolSize: 20, // 连接池大小
	})

	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	Rdb = rdb
	return nil
}

func HGetAllByKey(key string) map[string]string {
	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
	m, err := Rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return map[string]string{}
	}
	return m
}
