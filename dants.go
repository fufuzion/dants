package dants

import (
	"context"
	"errors"
	"github.com/fufuzion/dants/limiter"
	"sync"
)

var (
	mu             = &sync.RWMutex{}
	limiterM       = make(map[string]limiter.Limiter)
	ErrTaskExpired = errors.New("task expired")
)

func Register(key string, opts ...limiter.Option) error {
	lt, err := limiter.NewSimpleLimiter(opts...)
	if err != nil {
		return err
	}
	mu.Lock()
	limiterM[key] = lt
	mu.Unlock()
	return nil
}

func UnRegister(key string) {
	mu.Lock()
	delete(limiterM, key)
	mu.Unlock()
}

type taskArg struct {
	ctx context.Context
	Arg any
}
type ErrData struct {
	Arg any
	Err error
}
