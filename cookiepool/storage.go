package cookiepool

// 应用中，需要使用Cookie池的应用是登录应用。存储模块包括账号信息和Cookies信息。
// 账号由用户名和密码两部分组成，我们可以存成用户名和密码的映射。Cookies可以存成
// JSON字符串，并根据账号来生成Cookies。生成的时候我们需要账号是否已经生成了Cookies。
// 我们应用Redis的Hash存储账号信息和Cookies。

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

type Storage struct {
	rdb *redis.Client // redis客户端

	accountKey string // Redis的AccountKey
	cookieKey  string // Redis的CookieKey
}

func NewStorage(addr string, password string, keys ...string) (*Storage, error) {
	var accountKey string
	var cookieKey string

	switch len(keys) {
	case 1:
		accountKey = fmt.Sprintf("account:%s", keys[0])
		cookieKey = fmt.Sprintf("cookie:%s", keys[0])
	case 2:
		accountKey = fmt.Sprintf("account:%s", keys[0])
		cookieKey = fmt.Sprintf("cookie:%s", keys[1])
	default:
		return nil, fmt.Errorf("the length of keys must be 1 or 2: get len(keys) = %d", len(keys))
	}

	s := Storage{
		rdb: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
			PoolSize: 100,
		}),

		accountKey: accountKey,
		cookieKey:  cookieKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.rdb.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *Storage) set(key string, values ...any) error {
	ctx := context.Background()
	return s.rdb.HSet(ctx, key, values...).Err()
}

func (s *Storage) get(key, field string) (string, error) {
	ctx := context.Background()
	return s.rdb.HGet(ctx, key, field).Result()
}

func (s *Storage) delete(key string, fields ...string) error {
	ctx := context.Background()
	return s.rdb.HDel(ctx, key, fields...).Err()
}

func (s *Storage) count(key string) (int64, error) {
	ctx := context.Background()
	return s.rdb.HLen(ctx, key).Result()
}

func (s *Storage) getall(key string) (map[string]string, error) {
	ctx := context.Background()
	return s.rdb.HGetAll(ctx, key).Result()
}

func (s *Storage) SetAccount(username, value string) error {
	return s.set(s.accountKey, username, value)
}

func (s *Storage) SetCookie(username, value string) error {
	return s.set(s.cookieKey, username, value)
}

func (s *Storage) GetAccount(username string) (string, error) {
	return s.get(s.accountKey, username)
}

func (s *Storage) GetCookie(username string) (string, error) {
	return s.get(s.cookieKey, username)
}

func (s *Storage) DeleteAccount(usernames ...string) error {
	return s.delete(s.accountKey, usernames...)
}

func (s *Storage) DeleteCookie(usernames ...string) error {
	return s.delete(s.cookieKey, usernames...)
}

func (s *Storage) CountAccount() (int64, error) {
	return s.count(s.accountKey)
}

func (s *Storage) CountCookie() (int64, error) {
	return s.count(s.cookieKey)
}

// 随机获取网站Cookie
func (s *Storage) Random() (string, error) {
	ctx := context.Background()
	strs, err := s.rdb.HVals(ctx, s.cookieKey).Result()
	if err != nil {
		return "", err
	}
	if len(strs) > 0 {
		return strs[rand.Intn(len(strs))], nil
	}
	return "", fmt.Errorf("no cookie for key: %s", s.cookieKey)
}

func (s *Storage) GetAllAccount() (map[string]string, error) {
	return s.getall(s.accountKey)
}

func (s *Storage) GetAllCookie() (map[string]string, error) {
	return s.getall(s.cookieKey)
}

func (s *Storage) Usernames() ([]string, error) {
	ctx := context.Background()
	return s.rdb.HKeys(ctx, s.accountKey).Result()
}
