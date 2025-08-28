package limiter

import (
	"context"
	"errors"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/redis"
	"time"
)

const defaultLimiterPrefix = "dants:limiter:"

type Limiter interface {
	Take(ctx context.Context, key string) (bool, error)
	BlockTake(ctx context.Context, key string) (bool, error)
}

func NewSimpleLimiter(opts ...Option) (Limiter, error) {
	o := &option{}
	for _, opt := range opts {
		opt(o)
	}
	return newRedisLimiter(o)
}

type redisLimiter struct {
	o   *option
	ins *limiter.Limiter
}

func newRedisLimiter(o *option) (Limiter, error) {
	if o.rds == nil {
		return nil, errors.New("redis store can't be nil")
	}
	if o.limit <= 0 {
		return nil, errors.New("limit can't less than or equal to 0")
	}
	if o.period <= 0 {
		return nil, errors.New("period can't less than or equal to 0")
	}
	store, err := redis.NewStoreWithOptions(o.rds, limiter.StoreOptions{
		Prefix:          defaultLimiterPrefix,
		CleanUpInterval: limiter.DefaultCleanUpInterval,
		MaxRetry:        limiter.DefaultMaxRetry,
	})
	if err != nil {
		return nil, err
	}
	rate := limiter.Rate{
		Period: o.period,
		Limit:  o.limit,
	}
	instance := limiter.New(store, rate)
	return &redisLimiter{
		o:   o,
		ins: instance,
	}, nil
}
func (l *redisLimiter) Take(ctx context.Context, key string) (bool, error) {
	res, err := l.ins.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return !res.Reached, nil
}

func (l *redisLimiter) BlockTake(ctx context.Context, key string) (bool, error) {
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			res, err := l.ins.Get(ctx, key)
			if err != nil {
				return false, err
			}
			if !res.Reached {
				return true, nil
			}
			reset := time.Unix(res.Reset, 0)
			sleep := time.Until(reset)
			if sleep <= 0 {
				continue
			}
			time.Sleep(sleep)
			//select {
			//case <-ctx.Done():
			//	return false, ctx.Err()
			//case <-time.After(sleep):
			//	continue
			//}
		}
	}
}
