package proxypool

// 存储模块使用Redis的有序集合，用来做代理的去重和状态标识。

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	maxStorageScore  = 100.0
	minStorageScore  = 0.0
	initStorageScore = 10.0
)

type Storage struct {
	rdb *redis.Client // redis客户端
	key string        // 数据库键
}

func NewStorage(addr string, password string, key string) (*Storage, error) {
	s := Storage{
		rdb: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
			PoolSize: 100,
		}),
		key: key,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// 添加代理到数据库中，并设定分数
func (s *Storage) Add(proxy string, args ...float64) error {
	ctx := context.Background()
	_, err := s.rdb.ZScore(ctx, s.key, proxy).Result()
	if err != nil {
		score := initStorageScore
		if len(args) > 0 {
			sc := args[0]
			if sc < minStorageScore || sc > maxStorageScore {
				return fmt.Errorf("Add proxy failed: score must in range [%v, %v]", minStorageScore, maxStorageScore)
			}
			score = sc
		}
		return s.rdb.ZAdd(ctx, s.key, &redis.Z{Score: score, Member: proxy}).Err()
	}
	return nil
}

// 获取最高得分的代理
func (s *Storage) Random() (string, error) {
	ctx := context.Background()
	score := strconv.FormatFloat(maxStorageScore, 'f', 1, 64)
	proxies, err := s.rdb.ZRangeByScore(ctx, s.key,
		&redis.ZRangeBy{Min: score, Max: score}).Result()
	if err != nil {
		return "", err
	}
	if len(proxies) > 0 {
		return proxies[rand.Intn(len(proxies))], nil
	}
	proxies, err = s.rdb.ZRevRange(ctx, s.key, 0, 100).Result()
	if err != nil {
		return "", err
	}
	if len(proxies) > 0 {
		return proxies[rand.Intn(len(proxies))], nil
	}
	return "", fmt.Errorf("no memory in key %s in the db", s.key)
}

// 减少给定代理的分数。如果代理的分数为最低分，则删除代理
func (s *Storage) Decrease(proxy string) error {
	ctx := context.Background()
	score, err := s.rdb.ZScore(ctx, s.key, proxy).Result()
	if err != nil {
		return err
	}
	if score >= minStorageScore+1.0 {
		err = s.rdb.ZIncrBy(ctx, s.key, -1.0, proxy).Err()
		if err != nil {
			return err
		}
	} else {
		return s.rdb.ZRem(ctx, s.key, proxy).Err()
	}
	return nil
}

// 判断所给的代理是否存在
func (s *Storage) Exists(proxy string) (bool, error) {
	ctx := context.Background()
	err := s.rdb.ZScore(ctx, s.key, proxy).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}

// 设置所给的代理最高得分
func (s *Storage) SetMax(proxy string) error {
	ctx := context.Background()
	return s.rdb.ZAdd(ctx, s.key, &redis.Z{Score: maxStorageScore, Member: proxy}).Err()
}

// 计算数据库中所有代理的数目
func (s *Storage) Count() (int64, error) {
	ctx := context.Background()
	return s.rdb.ZCard(ctx, s.key).Result()
}

// 获得数据库中所有的代理
func (s *Storage) GetAll() ([]string, error) {
	ctx := context.Background()
	proxies, err := s.rdb.ZRangeByScore(ctx, s.key, &redis.ZRangeBy{
		Min: strconv.FormatFloat(minStorageScore, 'f', 1, 64),
		Max: strconv.FormatFloat(maxStorageScore, 'f', 1, 64),
	}).Result()
	return proxies, err
}

// 删除代理
func (s *Storage) Remove(proxies ...string) (bool, error) {
	ctx := context.Background()
	var proxiesI []interface{}
	for _, p := range proxies {
		proxiesI = append(proxiesI, p)
	}
	err := s.rdb.ZRem(ctx, s.key, proxiesI).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}
