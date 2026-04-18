# Multi-Terminal System Design Specification

## 项目概述

在浏览器中提供多主机实时终端能力，支持安全可控的批量命令执行（Multiexec），同时具备审计与风险防护能力。

**设计依据**: 严格遵循 `CLAUDE.md` 中的架构设计，所有实现必须符合文档中定义的协议、数据结构、行为规范。

---

## 一、技术栈

### 后端
- **语言**: Go (Golang)
- **SSH**: `golang.org/x/crypto/ssh`
- **WebSocket**: `github.com/gorilla/websocket`
- **配置**: Viper
- **压缩**: Zstd
- **缓存**: sync.Pool (分级缓冲池)

### 前端
- **框架**: Vue 3 + TypeScript
- **状态管理**: Pinia
- **终端**: xterm.js + xterm-addon-webgl
- **通信**: Web Worker + WebSocket

### 部署
- Kubernetes (K8s) - 完整优雅重启支持
- Linux (systemd) - 常规部署
- Docker - 开发与隔离部署

---

## 二、核心数据模型

### 2.1 Session (Go)

```go
type Session struct {
    SessionId       uint32                  // 帧协议路由的核心标识
    UserId          string                  // 归属的鉴权用户
    HostId          string                  // 绑定的目标资产标识
    SSHClient       *ssh.Client            // 底层网络连接句柄
    ShellStream     io.ReadWriteCloser     // PTY 的数据流
    Cols            int                     // 当前终端列数
    Rows            int                     // 当前终端行数
    State           SessionState            // 状态枚举：active, detached, expired, closed
    LastActiveAt    time.Time              // 最后活跃/心跳时间
    WriteBufferSize int                     // 衡量发送积压程度的水位线
    IsSlow          bool                    // 是否触发慢节点限流熔断
    PromptState     PromptState            // 上下文状态：shell, tui, unknown
}
```

### 2.2 ClientState (前端)

```typescript
interface ClientState {
  activeSessionId: number;
  selectedSessionIds: number[];
  mode: 'single' | 'multi';
  uiState: {
    echoLock: boolean;
    slowNodes: number[];
    tuiNodes: number[];
    disconnectedNodes: number[];
  };
}
```

### 2.3 CommandContext

```go
type CommandContext struct {
    Command            string
    RawInput           string
    Targets            []uint32
    UserId             string
    Timestamp          time.Time
    RiskLevel          RiskLevel  // low, medium, high
    ConsistencyCheckResult bool
}
```

---

## 三、二进制协议 (Big-Endian)

### 3.1 帧结构

```
+---------------+------------------+----------------------------------------+
| FrameType (1B)| SessionCount(1B) | Session Blocks (variable)              |
+---------------+------------------+----------------------------------------+

Session Block:
+-------------------+----------------+---------------------------+
| SessionId (4B)    | Length (2B)    | Data (Length Bytes)       |
+-------------------+----------------+---------------------------+
```

### 3.2 FrameType 定义

| 值  | 类型 | 方向 | 说明 |
|-----|------|------|------|
| 0x01 | Heartbeat | Bidirectional | 心跳保活，Data 可为空 |
| 0x02 | Terminal Output | Server→Client | PTY 标准输出流 |
| 0x03 | Terminal Input | Client→Server | 用户键盘/鼠标交互 |
| 0x04 | Window Resize | Client→Server | Cols/Rows (4B: 2+2) |
| 0x05 | Session Control | Bidirectional | 创建/销毁会话，JSON |

### 3.3 字节序

**统一使用 Big-Endian（网络字节序）**

---

## 四、后端核心组件

### 4.1 Session Manager

- 线程安全 `sync.Map` 注册表
- 状态机: `active → detached → expired → closed`
- 生命周期定时巡检 (清理 expired 会话)
- TTL: 10分钟 (detached 状态)

### 4.2 SSH Manager

- 密钥鉴权连接
- PTY 通道申请 (Read/Write 接口)
- SSH Keepalive 探测 (15秒无响应断开)

### 4.3 Stream Router (20ms 聚合引擎)

- 定时 Ticker，每 20ms 遍历活跃 Session
- 收割所有带数据的 Buffer
- 打包为单一 DataFrame 下发
- 错误回滚处理

### 4.4 Buffer Pool (分级缓冲池)

使用 `sync.Pool` 预分配：

| 规格 | 用途 |
|------|------|
| 256B | 小控制包 |
| 4KB | 普通数据包 |
| 32KB | 极限聚合包 |

**必须实现 `Reset()` 归还机制，防止 GC 压力**

### 4.5 Backpressure 流控

- 水位监测 (RingBuffer)
- 85% 水位: 标记 `isSlow = true`，暂停 SSH PTY Read
- 50% 水位: 恢复 Read
- 发送 SlowWarning 控制帧至前端

### 4.6 Context Checker (上下文一致性探测)

- 探测包: `\x05` (ENQ) 或 `\n`
- 状态识别:
  - `shell`: 包含 prompt (`$`/`#`)
  - `tui`: 全屏控制字符
  - `unknown`: 无响应 (2秒超时)
- tui/unknown 节点: 标红遮罩，禁止输入

### 4.7 Risk Control

三级管控：

| 级别 | 动作 | 行为 |
|------|------|------|
| Low | 监控 | 仅记录审计日志 |
| Medium | 确认 | 弹窗二次确认 |
| High | 阻断 | 直接拦截，输出红色警告 |

规则引擎: JSON 配置 + 预编译正则

### 4.8 Audit Service

- 结构化命令日志 (JSON)
- 敏感信息脱敏 (正则 `password`, `secret` 等)
- Zstd 流式压缩
- 分片对象存储 (S3/MinIO)

### 4.9 Resource Guard

- **启动预检**: `ulimit -n ≥ 4096` 否则拒绝启动
- **FD 监控**: 85% 水位触发降级模式
- **Rate Limiting**: 单 IP 并发 WS 请求限制

### 4.10 Graceful Restart

K8s/systemd 两套机制：

1. 接收 SIGUSR2 信号
2. 广播 "Maintenance" 控制帧
3. 拒绝新连接
4. Session 元数据序列化至 Redis
5. (K8s) Unix Domain Socket FD handoff
6. 新进程接管

---

## 五、前端核心组件

### 5.1 Web Worker 通信层

- 独立线程处理 WebSocket 二进制数据
- 使用 `DataView`/`Uint8Array` 解析
- 禁止 JSON fallback
- `postMessage` 与主线程通信

### 5.2 xterm.js 渲染

- 强制启用 WebGL 插件 (`xterm-addon-webgl`)
- Scrollback 上限: 5000 行
- WebGL fallback: 自动降级 Canvas

### 5.3 乐观回显与 EchoLock

```
1. 用户按键 → 本地立即渲染 (乐观回显)
2. 设置 EchoLock = true (全局锁)
3. 50ms 窗口内丢弃所有远端回显
4. 窗口结束，释放锁，正常渲染
```

### 5.4 requestAnimationFrame 节流

- Worker 数据先缓存到 Array 队列
- 绑定 rAF 钩子
- 16.6ms 内合并为单一字符串
- 一次性 `write()` 渲染

### 5.5 状态管理 (Pinia)

```typescript
// 每个终端 Wrapper 组件使用专属 Selector
const sessionState = useSessionStore(s => s.sessions[id])
// 仅监听自身 sessionId 对应状态
```

### 5.6 React.memo / 组件缓存

- 非激活终端不参与 Vue 渲染周期
- 精确状态订阅防止重绘

---

## 六、部署架构

### 6.1 Docker

```dockerfile
FROM golang:1.21-alpine
# 多阶段构建
# 前端资源注入
```

### 6.2 systemd

- 服务单元文件
- 环境变量配置
- 日志管理 (journald)

### 6.3 Kubernetes

- Deployment + Service
- HPA 弹性扩缩容
- PodDisruptionBudget (优雅关闭)
- PreStop Hook + SIGUSR2

---

## 七、可观测性

### 7.1 Prometheus Metrics

| 指标 | 说明 |
|------|------|
| `gateway_active_connections` | 活跃 WS 连接数 |
| `gateway_ssh_sessions_total` | SSH 隧道总数 |
| `gateway_fd_usage_percent` | FD 占用百分比 |
| `gateway_buffer_pool_hit_rate` | Pool 命中率 |
| `gateway_slow_nodes_total` | 慢节点数量 |

### 7.2 Alertmanager

- FD ≥ 85%: P1 告警
- SSH 会话 ≥ 80% 上限: 扩容警告

---

## 八、目录结构

```
multi-terminal/
├── gateway/
│   ├── cmd/server/
│   │   └── main.go
│   ├── internal/
│   │   ├── server/        # WS 服务器
│   │   ├── session/        # Session Manager
│   │   ├── ssh/           # SSH Manager
│   │   ├── router/        # Stream Router
│   │   ├── multiexec/     # Multiexec Engine
│   │   ├── diff/          # Diff Engine
│   │   ├── audit/         # Audit Service
│   │   ├── risk/          # Risk Control
│   │   ├── resource/      # Resource Guard
│   │   └── buffer/        # Buffer Pool
│   └── pkg/
│       └── codec/         # 二进制编解码
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── stores/
│   │   ├── workers/
│   │   └── utils/
│   └── public/
├── docker/
├── k8s/
├── Makefile
└── go.mod
```

---

## 九、阶段计划

| 阶段 | 任务 | 产出 |
|------|------|------|
| Phase 1 | 项目脚手架 + WS + 协议 | 可运行的 WS Echo 服务器 |
| Phase 2 | Session + SSH + xterm | 浏览器终端连接 SSH |
| Phase 3 | 流控 + 聚合 + Backpressure | 高并发稳定输出 |
| Phase 4 | Multiexec + 上下文探测 | 多机广播执行 |
| Phase 5 | Diff + Audit + 断连重连 | 审计与高可用 |
| Phase 6 | 测试 + CI/CD | 自动化验证 |
| Phase 7 | Prometheus + Grafana | 监控告警 |
| Phase 8 | 风控规则引擎 | 命令拦截 |
| Phase 9 | 前端性能优化 | WebGL + 节流 |
| Phase 10 | Resource Guard | 全局资源防护 |

---

## 十、质量标准

1. **内存安全**: 无 goroutine leak，无 FD leak
2. **协议严格**: Big-Endian，无 JSON fallback
3. **容错**: 数据截断保护，未知帧跳过
4. **安全**: 敏感信息脱敏，高危命令拦截
5. **性能**: Buffer 复用，20ms 聚合，rAF 节流
