# Dants (Distributed Rate-Limited Worker Pool)

`Distributed Rate-Limited Worker Pool` 该库提供了一个 **支持分布式限流** 的 **任务池 (Worker Pool)**，可用于高并发场景下的任务调度

## ✨ 功能特性
- **分布式限流**  
  多实例共享同一个 Redis Key 进行速率控制。
- **任务池 (Worker Pool)**    
  支持以类似ants的使用方式提交本地任务
- **支持上下文退出**  
  调用 `Invoke` 时可传入 `context.Context`，在超时或取消时立即停止等待。
- **错误处理回调**  
  支持自定义错误回调函数，统一处理任务失败、限流拒绝、任务超时等情况。    

## 🚀 安装

```bash
go get github.com/fufuzion/dants
```