package dants

import (
	"context"
	"errors"
	"fmt"
	"github.com/fufuzion/dants/limiter"
	"github.com/panjf2000/ants/v2"
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

func NewPoolWithFunc(key string, size int, pf func(any), errHandler func(data *ErrData)) (*Pool, error) {
	mu.RLock()
	lt, ok := limiterM[key]
	if !ok {
		mu.RUnlock()
		return nil, errors.New("limiter not exists")
	}
	mu.RUnlock()
	pool := &Pool{
		key:        key,
		limiter:    lt,
		errHandler: errHandler,
		wg:         &sync.WaitGroup{},
	}
	workPool, err := ants.NewPoolWithFunc(size, func(a any) {
		defer pool.wg.Done()
		task := a.(*Task)
		arg := task.Arg
		ok, err := pool.limiter.BlockTake(task.ctx, key)
		if err != nil {
			pool.tryErrHandler(&ErrData{Arg: arg, Err: err})
			return
		}
		if !ok {
			pool.tryErrHandler(&ErrData{Arg: arg, Err: ErrTaskExpired})
			return
		}
		defer func() {
			if r := recover(); r != nil {
				pool.tryErrHandler(&ErrData{Arg: arg, Err: fmt.Errorf("panic: %v", r)})
			}
		}()
		pf(arg)
	})
	if err != nil {
		return nil, err
	}
	pool.pool = workPool
	return pool, nil
}

type Task struct {
	ctx context.Context
	Arg any
}
type ErrData struct {
	Arg any
	Err error
}
type Pool struct {
	key        string
	pool       *ants.PoolWithFunc
	limiter    limiter.Limiter
	errHandler func(data *ErrData)
	wg         *sync.WaitGroup
}

func (d *Pool) tryErrHandler(data *ErrData) {
	if d.errHandler == nil {
		return
	}
	d.errHandler(data)
}

func (d *Pool) Release() {
	d.pool.Release()
}

func (d *Pool) Invoke(ctx context.Context, arg any) error {
	d.wg.Add(1)
	return d.pool.Invoke(&Task{
		ctx: ctx,
		Arg: arg,
	})
}

func (d *Pool) Wait() {
	d.wg.Wait()
}
