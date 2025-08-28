package dants

import (
	"context"
	"dants/limiter"
	"errors"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func Test_Register(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	// 正常注册
	err = Register("testKey",
		limiter.WithRedis(rdb),
		limiter.WithLimit(2),
		limiter.WithPeriod(time.Second),
	)
	assert.NoError(t, err)

	// 再次注册会覆盖
	err = Register("testKey",
		limiter.WithRedis(rdb),
		limiter.WithLimit(3),
		limiter.WithPeriod(time.Second),
	)
	assert.NoError(t, err)

	// 参数错误：limit <= 0
	err = Register("badKey",
		limiter.WithRedis(rdb),
		limiter.WithLimit(0),
		limiter.WithPeriod(time.Second),
	)
	assert.Error(t, err)

	// 参数错误：period <= 0
	err = Register("badKey2",
		limiter.WithRedis(rdb),
		limiter.WithLimit(1),
		limiter.WithPeriod(0),
	)
	assert.Error(t, err)
}

func Test_NewPoolWithFunc(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()
	closeCh := make(chan int)
	go func() {
		for {
			select {
			case <-closeCh:
				return
			default:
				time.Sleep(time.Second)
				mr.FastForward(time.Second)
			}
		}
	}()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	key := "testKey"
	err = Register(key,
		limiter.WithRedis(rdb),
		limiter.WithLimit(1),
		limiter.WithPeriod(time.Second),
	)
	assert.NoError(t, err)

	var results []string
	var gotErr error
	pool, err := NewPoolWithFunc(key, 5,
		func(arg any) {
			results = append(results, arg.(string))
		},
		func(data *ErrData) {
			gotErr = data.Err
		},
	)
	assert.NoError(t, err)
	defer pool.Release()

	err = pool.Invoke(context.Background(), "task1")
	assert.NoError(t, err)

	err = pool.Invoke(context.Background(), "task2")
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err = pool.Invoke(ctx, "task3")
	if err != nil {
		assert.True(t, errors.Is(err, context.DeadlineExceeded))
	}

	pool.Wait()

	assert.Contains(t, results, "task1")
	assert.Contains(t, results, "task2")
	assert.NotContains(t, results, "task3")
	close(closeCh)
	if gotErr != nil {
		fmt.Println("handled error:", gotErr)
	}
}

func Test_DistributedLimiter_Concurrent(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	// 模拟时间流逝
	stopCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				time.Sleep(200 * time.Millisecond)
				mr.FastForward(200 * time.Millisecond)
			}
		}
	}()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	limit := 3
	totalTimeout := 5

	key := "testKey"
	err = Register(key,
		limiter.WithRedis(rdb),
		limiter.WithLimit(int64(limit)),
		limiter.WithPeriod(time.Second),
	)
	assert.NoError(t, err)

	var success, fail atomic.Int32
	wg := sync.WaitGroup{}

	// 启动 concurrency 个 goroutine 模拟并发抢占
	concurrency := 20
	singleBatch := 10
	wg.Add(concurrency)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(totalTimeout)*time.Second)
	go func() {
		time.Sleep(time.Duration(totalTimeout) * time.Second)
		cancel()
	}()
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			p, err := NewPoolWithFunc(key, 10,
				func(arg any) {
					success.Add(1)
				},
				func(data *ErrData) {
					fail.Add(1)
				})
			assert.NoError(t, err)
			for j := 0; j < singleBatch; j++ {
				err = p.Invoke(ctx, fmt.Sprintf("task_%d_%d", id, j))
				assert.NoError(t, err)
			}
			p.Wait()
			p.Release()
		}(i)
	}

	wg.Wait()
	close(stopCh)

	fmt.Println("success:", success.Load(), "fail:", fail.Load())

	assert.GreaterOrEqual(t, success.Load(), int32(limit*totalTimeout)-1)
	assert.LessOrEqual(t, success.Load(), int32(limit*totalTimeout))
	assert.Equal(t, int32(concurrency*singleBatch), success.Load()+fail.Load())
}
