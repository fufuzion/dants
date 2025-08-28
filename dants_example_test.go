package dants

import (
	"context"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/fufuzion/dants/limiter"
	"github.com/redis/go-redis/v9"
	"time"
)

func Example() {
	// 使用redis作为store
	mr, _ := miniredis.Run()
	defer mr.Close()
	go func() {
		for {
			time.Sleep(time.Second)
			mr.FastForward(time.Second)
		}
	}()
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 注册限流器：每秒只允许 1 次
	err := Register("exampleKey",
		limiter.WithRedis(rdb),
		limiter.WithLimit(1),
		limiter.WithPeriod(time.Second),
	)
	if err != nil {
		panic(err)
	}

	var results []string
	pool, err := NewPoolWithFunc("exampleKey", 1,
		func(arg any) {
			results = append(results, arg.(string))
			fmt.Println("done:", arg.(string))
		},
		func(data *ErrData) {
			fmt.Println("error:", data.Arg, "err:", data.Err)
		},
	)
	if err != nil {
		panic(err)
	}

	// 提交多个任务
	ctx := context.Background()
	_ = pool.Invoke(ctx, "job1")
	_ = pool.Invoke(ctx, "job2")
	_ = pool.Invoke(ctx, "job3")

	// 等待所有任务完成
	pool.Wait()
	pool.Release()

	// Output:
	// done: job1
	// done: job2
	// done: job3
}
