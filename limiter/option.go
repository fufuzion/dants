package limiter

import (
	"github.com/ulule/limiter/v3/drivers/store/redis"
	"time"
)

type option struct {
	rds    redis.Client
	limit  int64
	period time.Duration
}
type Option func(*option)

func WithRedis(rds redis.Client) Option {
	return func(o *option) {
		o.rds = rds
	}
}
func WithLimit(limit int64) Option {
	return func(o *option) {
		o.limit = limit
	}
}

func WithPeriod(period time.Duration) Option {
	return func(o *option) {
		o.period = period
	}
}
