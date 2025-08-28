package dants

import (
	"context"
	"errors"
	"fmt"
	"github.com/fufuzion/dants/limiter"
	"github.com/panjf2000/ants/v2"
	"sync"
)

type Pool struct {
	key     string
	pool    *ants.Pool
	limiter limiter.Limiter
}

func NewPool(key string, size int, options ...ants.Option) (*Pool, error) {
	mu.RLock()
	lt, ok := limiterM[key]
	if !ok {
		mu.RUnlock()
		return nil, errors.New("limiter not exists")
	}
	mu.RUnlock()

	workPool, err := ants.NewPool(size, options...)
	if err != nil {
		return nil, err
	}
	return &Pool{
		key:     key,
		limiter: lt,
		pool:    workPool,
	}, nil
}

type Future struct {
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func (f *Future) Wait() error {
	<-f.done
	return f.getErr()
}

func (f *Future) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *Future) getErr() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

func (p *Pool) Submit(ctx context.Context, pf func(arg any), arg any) *Future {
	f := &Future{done: make(chan struct{})}
	wrapper := func() {
		defer close(f.done)
		ok, err := p.limiter.BlockTake(ctx, p.key)
		if err != nil {
			f.setErr(err)
			return
		}
		if !ok {
			f.setErr(ErrTaskExpired)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				f.setErr(fmt.Errorf("panic: %v", r))
			}
		}()
		pf(arg)
	}
	if err := p.pool.Submit(wrapper); err != nil {
		f.setErr(err)
		close(f.done)
	}
	return f
}
