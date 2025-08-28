package dants

import (
	"context"
	"errors"
	"fmt"
	"github.com/fufuzion/dants/limiter"
	"github.com/panjf2000/ants/v2"
	"sync"
)

func NewPoolWithFunc(key string, size int, pf func(any), errHandler func(data *ErrData), options ...ants.Option) (*PoolWithFunc, error) {
	mu.RLock()
	lt, ok := limiterM[key]
	if !ok {
		mu.RUnlock()
		return nil, errors.New("limiter not exists")
	}
	mu.RUnlock()
	pool := &PoolWithFunc{
		key:        key,
		limiter:    lt,
		errHandler: errHandler,
		wg:         &sync.WaitGroup{},
	}
	workPool, err := ants.NewPoolWithFunc(size, func(a any) {
		defer pool.wg.Done()
		task := a.(*taskArg)
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
	}, options...)
	if err != nil {
		return nil, err
	}
	pool.pool = workPool
	return pool, nil
}

type PoolWithFunc struct {
	key        string
	pool       *ants.PoolWithFunc
	limiter    limiter.Limiter
	errHandler func(data *ErrData)
	wg         *sync.WaitGroup
}

func (d *PoolWithFunc) tryErrHandler(data *ErrData) {
	if d.errHandler == nil {
		return
	}
	defer func() {
		recover()
	}()
	d.errHandler(data)
}

func (d *PoolWithFunc) Release() {
	d.pool.Release()
}

func (d *PoolWithFunc) Invoke(ctx context.Context, arg any) error {
	err := d.pool.Invoke(&taskArg{
		ctx: ctx,
		Arg: arg,
	})
	if err != nil {
		return err
	}
	d.wg.Add(1)
	return nil
}

func (d *PoolWithFunc) Wait() {
	d.wg.Wait()
}
