# Multi-Terminal System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现完整的浏览器多主机实时终端系统，支持 Multiexec 广播、审计、风控和流控

**Architecture:** Go 后端网关 + Vue 3 前端，二进制 WebSocket 协议，多路复用流控，资源看门狗

**Tech Stack:** Go + Gorilla WebSocket + crypto/ssh | Vue 3 + Pinia + xterm.js + Web Worker

---

## Phase 1: 基础设施与核心通信

### 文件结构

```
gateway/
├── cmd/server/main.go           # 入口
├── internal/
│   ├── server/server.go         # WS 服务器
│   └── codec/frame.go           # 二进制帧编解码
└── pkg/buffer/pool.go           # 分级缓冲池

frontend/
├── src/
│   ├── workers/ws.worker.ts      # Web Worker
│   └── workers/parser.ts        # 二进制解析
```

### 任务 1.1: Gateway 项目初始化

**Files:**
- Create: `gateway/go.mod`
- Create: `gateway/cmd/server/main.go`
- Create: `gateway/internal/config/config.go`
- Create: `gateway/Makefile`

- [ ] **Step 1: 创建 go.mod**

```bash
cd /home/mensyli1/Documents/workspace/multi-terminal
mkdir -p gateway/cmd/server gateway/internal/config gateway/pkg/buffer
cd gateway && go mod init github.com/mensylisir/multi-terminal/gateway
```

- [ ] **Step 2: 创建 main.go**

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/websocket"
    "github.com/spf13/viper"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
}

func main() {
    // 配置加载
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("server.read_timeout", 15*time.Second)
    viper.SetDefault("server.write_timeout", 15*time.Second)
    viper.AutomaticEnv()

    // 优雅停机
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        log.Println("收到退出信号，开始优雅关闭...")
        cancel()
    }()

    // 启动 WS 服务器
    mux := http.NewServeMux()
    mux.HandleFunc("/ws", handleWS)
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", viper.GetInt("server.port")),
        Handler:      mux,
        ReadTimeout:  viper.GetDuration("server.read_timeout"),
        WriteTimeout: viper.GetDuration("server.write_timeout"),
    }

    go func() {
        log.Printf("Gateway 启动监听 :%d", viper.GetInt("server.port"))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    <-ctx.Done()
    log.Println("Gateway 关闭完成")
}
```

- [ ] **Step 3: 创建配置模块**

```go
package config

import "github.com/spf13/viper"

type Config struct {
    Server   ServerConfig
    Redis    RedisConfig
    Resource ResourceConfig
}

type ServerConfig struct {
    Port           int
    ReadTimeout    int
    WriteTimeout   int
    IdleTimeout    int
}

type RedisConfig struct {
    Addr     string
    Password string
    DB       int
}

type ResourceConfig struct {
    MaxFDLimit    int
    FDDangerRatio float64
    MaxSessions   int
    MaxSessionsPerUser int
}

func Load() *Config {
    viper.SetDefault("server.port", 8080)
    viper.SetDefault("resource.max_fd_limit", 4096)
    viper.SetDefault("resource.fd_danger_ratio", 0.85)
    viper.SetDefault("resource.max_sessions", 2000)
    viper.SetDefault("resource.max_sessions_per_user", 20)
    viper.AutomaticEnv()
    return &Config{}
}
```

- [ ] **Step 4: 创建 Makefile**

```makefile
.PHONY: build run test clean docker

build:
	go build -o bin/gateway ./cmd/server

run: build
	./bin/gateway

test:
	go test -v ./...

clean:
	rm -rf bin/
```

- [ ] **Step 5: 提交**

```bash
cd /home/mensyli1/Documents/workspace/multi-terminal
git add gateway/
git commit -m "feat(gateway): initialize project with config and graceful shutdown"
```

---

### 任务 1.2: WebSocket Server 基础实现

**Files:**
- Create: `gateway/internal/server/server.go`
- Create: `gateway/internal/server/conn.go`
- Modify: `gateway/cmd/server/main.go` (添加 handleWS)

- [ ] **Step 1: 创建连接管理**

```go
package server

import (
    "sync"
    "time"
    "github.com/gorilla/websocket"
)

const (
    pingPeriod = 30 * time.Second
    pongWait   = 60 * time.Second
    writeWait  = 10 * time.Second
)

type Conn struct {
    ID     uint32
    WS     *websocket.Conn
    Send   chan []byte
   mu     sync.Mutex
}

func NewConn(id uint32, ws *websocket.Conn) *Conn {
    return &Conn{
        ID:   id,
        WS:   ws,
        Send: make(chan []byte, 256),
    }
}

func (c *Conn) ReadPump(handler func([]byte)) {
    defer func() {
        c.WS.Close()
        close(c.Send)
    }()
    c.WS.SetReadLimit(65535)
    c.WS.SetReadDeadline(time.Now().Add(pongWait))
    c.WS.SetPongHandler(func(string) error {
        c.WS.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })
    for {
        _, msg, err := c.WS.ReadMessage()
        if err != nil {
            break
        }
        handler(msg)
    }
}

func (c *Conn) WritePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.WS.Close()
        close(c.Send)
    }()
    for {
        select {
        case msg, ok := <-c.Send:
            c.WS.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                c.WS.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            c.mu.Lock()
            err := c.WS.WriteMessage(websocket.BinaryMessage, msg)
            c.mu.Unlock()
            if err != nil {
                return
            }
        case <-ticker.C:
            c.WS.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.WS.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

- [ ] **Step 2: 创建 Hub (连接中枢)**

```go
package server

import (
    "sync"
    "sync/atomic"
)

type Hub struct {
    IDGen    uint32
    ConnMap   sync.Map
    Register  chan *Conn
    Unregister chan *Conn
}

func NewHub() *Hub {
    return &Hub{
        Register:   make(chan *Conn),
        Unregister: make(chan *Conn),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case conn := <-h.Register:
            atomic.StoreUint32(&conn.ID, atomic.AddUint32(&h.IDGen, 1))
            h.ConnMap.Store(conn.ID, conn)
        case conn := <-h.Unregister:
            h.ConnMap.Delete(conn.ID)
            close(conn.Send)
        }
    }
}
```

- [ ] **Step 3: 创建 handleWS**

```go
package server

import (
    "net/http"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

func handleWS(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    conn := NewConn(0, ws)
    HubGlobal.Register <- conn
    go conn.WritePump()
    go conn.ReadPump(func(msg []byte) {
        // Echo 简单回显测试
        conn.Send <- msg
    })
}
```

- [ ] **Step 4: 修改 main.go 集成 Hub**

```go
// 在 main.go 添加
var HubGlobal = server.NewHub()

func init() {
    go HubGlobal.Run()
}
```

- [ ] **Step 5: 提交**

```bash
git add gateway/internal/server/
git commit -m "feat(gateway): add WebSocket server with ping/pong heartbeat"
```

---

### 任务 1.3: 二进制协议编解码器开发

**Files:**
- Create: `gateway/pkg/codec/frame.go`
- Create: `gateway/pkg/codec/frame_test.go`

- [ ] **Step 1: 定义帧类型常量**

```go
package codec

const (
    FrameTypeHeartbeat       = 0x01
    FrameTypeTerminalOutput  = 0x02
    FrameTypeTerminalInput   = 0x03
    FrameTypeWindowResize    = 0x04
    FrameTypeSessionControl  = 0x05
)
```

- [ ] **Step 2: 创建帧结构**

```go
package codec

import (
    "encoding/binary"
    "errors"
)

var (
    ErrBufferTooSmall = errors.New("buffer too small")
    ErrInvalidFrame   = errors.New("invalid frame")
)

type Frame struct {
    Type         uint8
    SessionCount uint8
    Sessions     []SessionBlock
}

type SessionBlock struct {
    SessionID uint32
    Length    uint16
    Data      []byte
}

func (f *Frame) Serialize() []byte {
    // 计算总长度
    totalLen := 2 // FrameType + SessionCount
    for _, s := range f.Sessions {
        totalLen += 4 + 2 + len(s.Data) // SessionID + Length + Data
    }
    buf := make([]byte, totalLen)
    buf[0] = f.Type
    buf[1] = f.SessionCount
    offset := 2
    for _, s := range f.Sessions {
        binary.BigEndian.PutUint32(buf[offset:], s.SessionID)
        offset += 4
        binary.BigEndian.PutUint16(buf[offset:], s.Length)
        offset += 2
        copy(buf[offset:], s.Data)
        offset += int(s.Length)
    }
    return buf
}

func Deserialize(data []byte) (*Frame, error) {
    if len(data) < 2 {
        return nil, ErrBufferTooSmall
    }
    frameType := data[0]
    sessionCount := data[1]
    offset := 2
    sessions := make([]SessionBlock, 0, sessionCount)
    for i := uint8(0); i < sessionCount; i++ {
        if offset+6 > len(data) {
            return nil, ErrBufferTooSmall
        }
        sessionID := binary.BigEndian.Uint32(data[offset:])
        offset += 4
        length := binary.BigEndian.Uint16(data[offset:])
        offset += 2
        if offset+int(length) > len(data) {
            return nil, ErrBufferTooSmall
        }
        session := SessionBlock{
            SessionID: sessionID,
            Length:    length,
            Data:      make([]byte, length),
        }
        copy(session.Data, data[offset:offset+int(length)])
        offset += int(length)
        sessions = append(sessions, session)
    }
    return &Frame{
        Type:         frameType,
        SessionCount: sessionCount,
        Sessions:     sessions,
    }, nil
}
```

- [ ] **Step 3: 创建 Buffer Pool**

```go
package buffer

import (
    "sync"
)

type Pool struct {
    small   sync.Pool // 256B
    medium  sync.Pool // 4KB
    large   sync.Pool // 32KB
}

func NewPool() *Pool {
    return &Pool{
        small: sync.Pool{
            New: func() interface{} {
                return make([]byte, 256)
            },
        },
        medium: sync.Pool{
            New: func() interface{} {
                return make([]byte, 4096)
            },
        },
        large: sync.Pool{
            New: func() interface{} {
                return make([]byte, 32768)
            },
        },
    }
}

func (p *Pool) Get(size int) []byte {
    switch {
    case size <= 256:
        return p.small.Get().([]byte)[:0]
    case size <= 4096:
        return p.medium.Get().([]byte)[:0]
    default:
        return p.large.Get().([]byte)[:0]
    }
}

func (p *Pool) Put(buf []byte) {
    switch cap(buf) {
    case 256:
        buf = buf[:256]
        p.small.Put(buf)
    case 4096:
        buf = buf[:4096]
        p.medium.Put(buf)
    case 32768:
        buf = buf[:32768]
        p.large.Put(buf)
    }
}
```

- [ ] **Step 4: 编写测试**

```go
package codec

import (
    "testing"
)

func TestFrameSerializeDeserialize(t *testing.T) {
    frame := &Frame{
        Type:         FrameTypeTerminalOutput,
        SessionCount: 1,
        Sessions: []SessionBlock{
            {
                SessionID: 12345,
                Length:    5,
                Data:      []byte("hello"),
            },
        },
    }
    data := frame.Serialize()
    parsed, err := Deserialize(data)
    if err != nil {
        t.Fatalf("Deserialize failed: %v", err)
    }
    if parsed.Type != frame.Type {
        t.Errorf("Type mismatch: got %d, want %d", parsed.Type, frame.Type)
    }
    if parsed.SessionCount != frame.SessionCount {
        t.Errorf("SessionCount mismatch: got %d, want %d", parsed.SessionCount, frame.SessionCount)
    }
    if len(parsed.Sessions) != 1 {
        t.Fatalf("Sessions length: got %d, want 1", len(parsed.Sessions))
    }
    if parsed.Sessions[0].SessionID != 12345 {
        t.Errorf("SessionID mismatch")
    }
    if string(parsed.Sessions[0].Data) != "hello" {
        t.Errorf("Data mismatch: got %s", string(parsed.Sessions[0].Data))
    }
}
```

- [ ] **Step 5: 运行测试**

```bash
cd gateway && go test -v ./pkg/codec/...
```

- [ ] **Step 6: 提交**

```bash
git add gateway/pkg/codec/ gateway/pkg/buffer/
git commit -m "feat(gateway): add binary protocol codec and buffer pool"
```

---

### 任务 1.4: 前端核心通信层与 Web Worker

**Files:**
- Create: `frontend/index.html`
- Create: `frontend/src/workers/ws.worker.ts`
- Create: `frontend/src/workers/parser.ts`
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`

- [ ] **Step 1: 初始化前端项目**

```bash
cd /home/mensyli1/Documents/workspace/multi-terminal
mkdir -p frontend/src/workers frontend/src/components frontend/src/stores
cd frontend && npm init -y
npm install --save-dev vite typescript vue-tsc @vitejs/plugin-vue
npm install vue pinia xterm xterm-addon-webgl
```

- [ ] **Step 2: 创建 package.json**

```json
{
  "name": "multi-terminal-frontend",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "pinia": "^2.1.0",
    "xterm": "^5.3.0",
    "xterm-addon-webgl": "^0.16.0"
  },
  "devDependencies": {
    "vite": "^5.0.0",
    "typescript": "^5.3.0",
    "vue-tsc": "^1.8.0",
    "@vitejs/plugin-vue": "^5.0.0"
  }
}
```

- [ ] **Step 3: 创建 Web Worker 解析器**

```typescript
// frontend/src/workers/parser.ts

export const FrameType = {
  Heartbeat: 0x01,
  TerminalOutput: 0x02,
  TerminalInput: 0x03,
  WindowResize: 0x04,
  SessionControl: 0x05,
} as const;

export interface ParsedFrame {
  type: number;
  sessionCount: number;
  sessions: SessionBlock[];
}

export interface SessionBlock {
  sessionId: number;
  length: number;
  data: Uint8Array;
}

export function parseFrame(buffer: ArrayBuffer): ParsedFrame | null {
  if (buffer.byteLength < 2) return null;

  const view = new DataView(buffer);
  const type = view.getUint8(0);
  const sessionCount = view.getUint8(1);
  let offset = 2;

  const sessions: SessionBlock[] = [];

  for (let i = 0; i < sessionCount; i++) {
    if (offset + 6 > buffer.byteLength) return null;

    const sessionId = view.getUint32(offset, false); // BigEndian
    offset += 4;
    const length = view.getUint16(offset, false);
    offset += 2;

    if (offset + length > buffer.byteLength) return null;

    const data = new Uint8Array(buffer, offset, length);
    sessions.push({ sessionId, length, data });
    offset += length;
  }

  return { type, sessionCount, sessions };
}
```

- [ ] **Step 4: 创建 Web Worker**

```typescript
// frontend/src/workers/ws.worker.ts

import { parseFrame, FrameType } from './parser';

interface WorkerMessage {
  type: 'send' | 'connect' | 'disconnect';
  payload?: any;
}

let ws: WebSocket | null = null;

self.onmessage = (e: MessageEvent<WorkerMessage>) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'connect':
      ws = new WebSocket(payload.url);
      ws.binaryType = 'arraybuffer';
      ws.onmessage = (event) => {
        const frame = parseFrame(event.data);
        if (frame) {
          self.postMessage({ type: 'frame', payload: frame });
        }
      };
      ws.onclose = () => {
        self.postMessage({ type: 'closed' });
      };
      break;

    case 'send':
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(payload);
      }
      break;

    case 'disconnect':
      if (ws) {
        ws.close();
        ws = null;
      }
      break;
  }
};
```

- [ ] **Step 5: 创建 HTML**

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>Multi-Terminal</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
```

- [ ] **Step 6: 创建 vite.config.ts**

```typescript
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
  },
  build: {
    target: 'esnext',
  },
});
```

- [ ] **Step 7: 创建 tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "lib": ["ESNext", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "preserve",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["src/**/*.ts", "src/**/*.d.ts"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 8: 创建 main.ts**

```typescript
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import App from './App.vue';

const app = createApp(App);
app.use(createPinia());
app.mount('#app');
```

- [ ] **Step 9: 创建 App.vue**

```vue
<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="status">{{ connected ? '已连接' : '未连接' }}</div>
    <button @click="connect">连接</button>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';

const connected = ref(false);
let worker: Worker | null = null;

function connect() {
  worker = new Worker(new URL('./workers/ws.worker.ts', import.meta.url), {
    type: 'module',
  });
  worker.onmessage = (e) => {
    if (e.data.type === 'frame') {
      console.log('Received frame:', e.data.payload);
    } else if (e.data.type === 'closed') {
      connected.value = false;
    }
  };
  worker.postMessage({ type: 'connect', payload: { url: 'ws://localhost:8080/ws' } });
  connected.value = true;
}

onUnmounted(() => {
  worker?.postMessage({ type: 'disconnect' });
});
</script>

<style>
.container {
  padding: 20px;
}
.status {
  margin: 10px 0;
}
</style>
```

- [ ] **Step 10: 提交**

```bash
git add frontend/
git commit -m "feat(frontend): initialize Vue 3 project with Web Worker"
```

---

## Phase 1 完成检查清单

- [ ] Gateway 可通过 `make build` 编译
- [ ] WebSocket 心跳机制工作
- [ ] 二进制帧编解码测试通过
- [ ] Buffer Pool 256B/4KB/32KB 规格正常
- [ ] 前端 Vite dev server 可启动
- [ ] Web Worker 与主线程消息通信正常

---

## Phase 2: 会话管理与终端接入

### 任务 2.1: Session Manager 模块

**Files:**
- Create: `gateway/internal/session/manager.go`
- Create: `gateway/internal/session/session.go`
- Create: `gateway/internal/session/state.go`

- [ ] **Step 1: 创建 Session 状态机**

```go
package session

type State int

const (
    StateActive    State = iota
    StateDetached
    StateExpired
    StateClosed
)

func (s State) String() string {
    switch s {
    case StateActive:
        return "active"
    case StateDetached:
        return "detached"
    case StateExpired:
        return "expired"
    case StateClosed:
        return "closed"
    default:
        return "unknown"
    }
}
```

- [ ] **Step 2: 创建 Session 结构体**

```go
package session

import (
    "io"
    "sync"
    "time"

    "golang.org/x/crypto/ssh"
)

type PromptState int

const (
    PromptShell   PromptState = iota
    PromptTUI
    PromptUnknown
)

type Session struct {
    SessionID       uint32
    UserID          string
    HostID          string
    SSHClient       *ssh.Client
    ShellStream     io.ReadWriteCloser
    Cols            int
    Rows            int
    State           State
    LastActiveAt    time.Time
    WriteBufferSize int
    IsSlow          bool
    PromptState     PromptState
    mu              sync.RWMutex
}

func NewSession(id uint32, userID, hostID string) *Session {
    return &Session{
        SessionID:    id,
        UserID:       userID,
        HostID:       hostID,
        Cols:         80,
        Rows:         24,
        State:        StateActive,
        LastActiveAt: time.Now(),
        PromptState:  PromptUnknown,
    }
}

func (s *Session) SetState(state State) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.State = state
}

func (s *Session) GetState() State {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.State
}

func (s *Session) SetPromptState(ps PromptState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.PromptState = ps
}
```

- [ ] **Step 3: 创建 Manager**

```go
package session

import (
    "sync"
    "time"
)

const (
    DefaultTTL = 10 * time.Minute
    CleanupInterval = 1 * time.Minute
)

type Manager struct {
    sessions   sync.Map
    ttl        time.Duration
    cleanupTick *time.Ticker
    stopCleanup chan struct{}
}

func NewManager() *Manager {
    m := &Manager{
        ttl:        DefaultTTL,
        stopCleanup: make(chan struct{}),
    }
    go m.cleanupLoop()
    return m
}

func (m *Manager) cleanupLoop() {
    m.cleanupTick = time.NewTicker(CleanupInterval)
    for {
        select {
        case <-m.cleanupTick.C:
            m.cleanup()
        case <-m.stopCleanup:
            m.cleanupTick.Stop()
            return
        }
    }
}

func (m *Manager) cleanup() {
    m.sessions.Range(func(key, value any) bool {
        s := value.(*Session)
        if s.GetState() == StateDetached && time.Since(s.LastActiveAt) > m.ttl {
            m.sessions.Delete(key)
        }
        return true
    })
}

func (m *Manager) Register(s *Session) {
    m.sessions.Store(s.SessionID, s)
}

func (m *Manager) Unregister(id uint32) {
    m.sessions.Delete(id)
}

func (m *Manager) Get(id uint32) (*Session, bool) {
    v, ok := m.sessions.Load(id)
    if !ok {
        return nil, false
    }
    return v.(*Session), true
}

func (m *Manager) Range(f func(*Session) bool) {
    m.sessions.Range(func_, value any) bool {
        return f(value.(*Session))
    })
}

func (m *Manager) Stop() {
    close(m.stopCleanup)
}
```

- [ ] **Step 4: 提交**

```bash
git add gateway/internal/session/
git commit -m "feat(gateway): add Session Manager with state machine and TTL cleanup"
```

---

### 任务 2.2: SSH Manager 模块

**Files:**
- Create: `gateway/internal/ssh/client.go`
- Create: `gateway/internal/ssh/manager.go`

- [ ] **Step 1: 创建 SSH Client**

```go
package ssh

import (
    "fmt"
    "io"
    "time"

    "golang.org/x/crypto/ssh"
)

type Client struct {
    Config *ssh.ClientConfig
    Client *ssh.Client
}

func NewClientConfig(user string, authMethods ...ssh.AuthMethod) *ssh.ClientConfig {
    return &ssh.ClientConfig{
        User:            user,
        Auth:            authMethods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         30 * time.Second,
    }
}

func (c *Client) Connect(addr string) error {
    client, err := ssh.Dial("tcp", addr, c.Config)
    if err != nil {
        return fmt.Errorf("ssh dial failed: %w", err)
    }
    c.Client = client
    return nil
}

func (c *Client) NewSession() (*ssh.Session, error) {
    if c.Client == nil {
        return nil, fmt.Errorf("client not connected")
    }
    return c.Client.NewSession()
}

func (c *Client) OpenPTY(cols, rows int) (io.ReadWriteCloser, error) {
    session, err := c.NewSession()
    if err != nil {
        return nil, err
    }

    fd, win, err := session.Pty()
    if err != nil {
        return nil, err
    }

    win.Width = cols
    win.Height = rows
    session.RequestPty("xterm-256color", cols, rows, win)

    stdin, err := session.StdinPipe()
    if err != nil {
        return nil, err
    }

    stdout, err := session.StdoutPipe()
    if err != nil {
        return nil, err
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        return nil, err
    }

    if err := session.Shell(); err != nil {
        return nil, err
    }

    return &ptySession{
        stdin:  stdin,
        stdout: stdout,
        stderr: stderr,
        session: session,
    }, nil
}

type ptySession struct {
    stdin  io.WriteCloser
    stdout io.Reader
    stderr io.Reader
    session *ssh.Session
}

func (p *ptySession) Read(b []byte) (int, error) {
    return p.stdout.Read(b)
}

func (p *ptySession) Write(b []byte) (int, error) {
    return p.stdin.Write(b)
}

func (p *ptySession) Close() error {
    p.session.Close()
    return nil
}

func (p *ptySession) Close() error {
    p.stdin.Close()
    return p.session.Close()
}
```

- [ ] **Step 2: 创建 SSH Manager**

```go
package ssh

import (
    "fmt"
    "sync"
    "time"

    "golang.org/x/crypto/ssh"
)

type Manager struct {
    clients sync.Map
    kaTick  int32
}

func NewManager() *Manager {
    m := &Manager{}
    go m.keepaliveLoop()
    return m
}

func (m *Manager) keepaliveLoop() {
    ticker := time.NewTicker(15 * time.Second)
    for range ticker.C {
        m.clients.Range(func(key, value any) bool {
            client := value.(*Client)
            if client.Client != nil {
                client.Client.SendRequest("keepalive@gateway.com", true, nil)
            }
            return true
        })
    }
}

func (m *Manager) GetOrCreate(host string, config *ssh.ClientConfig) (*Client, error) {
    v, ok := m.clients.Load(host)
    if ok {
        return v.(*Client), nil
    }
    client := &Client{Config: config}
    if err := client.Connect(host); err != nil {
        return nil, err
    }
    m.clients.Store(host, client)
    return client, nil
}

func (m *Manager) Close(host string) error {
    v, ok := m.clients.Load(host)
    if !ok {
        return fmt.Errorf("client not found")
    }
    client := v.(*Client)
    if client.Client != nil {
        client.Client.Close()
    }
    m.clients.Delete(host)
    return nil
}
```

- [ ] **Step 3: 修复 ptySession Close 重复定义问题**

```go
// 删除重复的 Close 方法，保留一个
func (p *ptySession) Close() error {
    p.stdin.Close()
    return p.session.Close()
}
```

- [ ] **Step 4: 提交**

```bash
git add gateway/internal/ssh/
git commit -m "feat(gateway): add SSH Manager with PTY support and keepalive"
```

---

### 任务 2.3: 前端 xterm.js 渲染接入

**Files:**
- Create: `frontend/src/components/Terminal.vue`
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: 创建 Terminal 组件**

```vue
<template>
  <div ref="terminalRef" class="terminal-container"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue';
import { Terminal } from 'xterm';
import { WebglAddon } from 'xterm-addon-webgl';
import 'xterm/css/xterm.css';

const props = defineProps<{
  sessionId: number;
}>();

const terminalRef = ref<HTMLElement | null>(null);
let terminal: Terminal | null = null;
let webglAddon: WebglAddon | null = null;

function initTerminal() {
  if (!terminalRef.value) return;

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
    },
    rows: 24,
    cols: 80,
    scrollback: 5000,
  });

  // 尝试启用 WebGL
  try {
    webglAddon = new WebglAddon();
    terminal.loadAddon(webglAddon);
  } catch (e) {
    console.warn('WebGL not available, falling back to canvas');
  }

  terminal.open(terminalRef.value);
}

function writeToTerminal(data: string) {
  terminal?.write(data);
}

function resize(cols: number, rows: number) {
  terminal?.resize(cols, rows);
}

defineExpose({ writeToTerminal, resize });

onMounted(() => {
  initTerminal();
});

onUnmounted(() => {
  terminal?.dispose();
  webglAddon?.dispose();
});
</script>

<style scoped>
.terminal-container {
  width: 100%;
  height: 100%;
  min-height: 400px;
}
</style>
```

- [ ] **Step 2: 更新 App.vue**

```vue
<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="status">{{ connected ? '已连接' : '未连接' }}</div>
    <button @click="connect">连接测试</button>
    <div class="terminal-grid">
      <Terminal :sessionId="1" ref="term1" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import Terminal from './components/Terminal.vue';

const connected = ref(false);
const term1 = ref<InstanceType<typeof Terminal> | null>(null);

function connect() {
  connected.value = true;
  // 模拟输出
  setTimeout(() => {
    term1.value?.writeToTerminal('Welcome to Multi-Terminal\r\n$ ');
  }, 100);
}
</script>

<style>
.container {
  padding: 20px;
}
.status {
  margin: 10px 0;
  font-weight: bold;
}
.terminal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(600px, 1fr));
  gap: 10px;
  margin-top: 20px;
}
button {
  padding: 8px 16px;
  background: #0066cc;
  color: white;
  border: none;
  cursor: pointer;
}
</style>
```

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/ frontend/src/App.vue
git commit -m "feat(frontend): add xterm.js with WebGL support"
```

---

### 任务 2.4: 终端窗口自适应控制

**Files:**
- Modify: `frontend/src/components/Terminal.vue`
- Modify: `frontend/src/workers/ws.worker.ts`

- [ ] **Step 1: 添加 resize 监听**

```typescript
// 在 Terminal.vue 中添加
import { onMounted, onUnmounted } from 'vue';

function handleResize() {
  if (terminalRef.value) {
    const cols = Math.floor(terminalRef.value.clientWidth / 9);
    const rows = Math.floor(terminalRef.value.clientHeight / 17);
    resize(cols, rows);
    // 发送 resize 帧到 worker
    emit('resize', { sessionId: props.sessionId, cols, rows });
  }
}

const emit = defineEmits(['resize']);

onMounted(() => {
  window.addEventListener('resize', handleResize);
});

onUnmounted(() => {
  window.removeEventListener('resize', handleResize);
});
```

- [ ] **Step 2: 添加 resize 帧处理**

```typescript
// 在 ws.worker.ts 中添加
case 'resize':
  const resizeFrame = createResizeFrame(payload.sessionId, payload.cols, payload.rows);
  ws?.send(resizeFrame);
  break;

// 添加 createResizeFrame 函数
function createResizeFrame(sessionId: number, cols: number, rows: number): ArrayBuffer {
  const buf = new ArrayBuffer(7);
  const view = new DataView(buf);
  view.setUint8(0, 0x04); // WindowResize
  view.setUint8(1, 1);   // SessionCount
  view.setUint32(2, sessionId, false); // BigEndian
  view.setUint16(6, cols, false);
  view.setUint16(8, rows, false);
  return buf;
}
```

- [ ] **Step 3: 提交**

```bash
git add frontend/src/
git commit -m "feat(frontend): add terminal resize handling"
```

---

## Phase 2 完成检查清单

- [ ] Session Manager 注册/查找/删除正常
- [ ] Session 状态机 (active→detached→expired→closed) 工作
- [ ] SSH Manager 可建立连接和申请 PTY
- [ ] xterm.js 渲染正常，WebGL 降级正常
- [ ] Terminal resize 事件触发并发送帧

---

## Phase 3: 聚合路由与流控缓冲

### 任务 3.1: 终端输出缓冲队列

**Files:**
- Create: `gateway/internal/router/buffer.go`

- [ ] **Step 1: 创建 RingBuffer**

```go
package router

import (
    "sync"
    "sync/atomic"
)

const (
    BufferSize   = 1024 * 1024 // 1MB
    WaterMarkHigh = BufferSize * 85 / 100
    WaterMarkLow  = BufferSize * 50 / 100
)

type RingBuffer struct {
    data   []byte
    write  atomic.Int64
    read   atomic.Int64
    mu     sync.Mutex
    cond   *sync.Cond
    closed bool
}

func NewRingBuffer() *RingBuffer {
    rb := &RingBuffer{
        data: make([]byte, BufferSize),
    }
    rb.cond = sync.NewCond(&rb.mu)
    return rb
}

func (rb *RingBuffer) Write(p []byte) (int, error) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    if rb.closed {
        return 0, io.EOF
    }

    n := copy(rb.data[rb.write.Load()%BufferSize:], p)
    rb.write.Add(int64(n))
    rb.cond.Signal()
    return n, nil
}

func (rb *RingBuffer) Read(p []byte) (int, error) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    for rb.read.Load() >= rb.write.Load() && !rb.closed {
        rb.cond.Wait()
    }

    if rb.read.Load() >= rb.write.Load() && rb.closed {
        return 0, io.EOF
    }

    n := copy(p, rb.data[rb.read.Load()%BufferSize:])
    rb.read.Add(int64(n))
    return n, nil
}

func (rb *RingBuffer) Available() int {
    return int(rb.write.Load() - rb.read.Load())
}

func (rb *RingBuffer) IsHigh() bool {
    return rb.Available() >= WaterMarkHigh
}

func (rb *RingBuffer) IsLow() bool {
    return rb.Available() <= WaterMarkLow
}

func (rb *RingBuffer) Close() {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    rb.closed = true
    rb.cond.Broadcast()
}
```

- [ ] **Step 2: 提交**

```bash
git add gateway/internal/router/buffer.go
git commit -m "feat(gateway): add RingBuffer with high/low watermark"
```

---

### 任务 3.2: Stream Router (20ms 聚合引擎)

**Files:**
- Create: `gateway/internal/router/router.go`
- Modify: `gateway/internal/server/server.go` (集成 Router)

- [ ] **Step 1: 创建 Router**

```go
package router

import (
    "log"
    "sync"
    "time"

    "github.com/mensylisir/multi-terminal/gateway/pkg/buffer"
    "github.com/mensylisir/multi-terminal/gateway/pkg/codec"
    "github.com/mensylisir/multi-terminal/gateway/internal/session"
)

const (
    AggregateInterval = 20 * time.Millisecond
)

type Router struct {
    buffers    sync.Map
    hub        *server.Hub
    sessionMgr *session.Manager
    ticker     *time.Ticker
    stop       chan struct{}
    pool       *buffer.Pool
}

func NewRouter(hub *server.Hub, sessionMgr *session.Manager, pool *buffer.Pool) *Router {
    return &Router{
        hub:        hub,
        sessionMgr: sessionMgr,
        pool:       pool,
        stop:       make(chan struct{}),
    }
}

func (r *Router) RegisterBuffer(sessionID uint32) *RingBuffer {
    rb := NewRingBuffer()
    r.buffers.Store(sessionID, rb)
    return rb
}

func (r *Router) UnregisterBuffer(sessionID uint32) {
    r.buffers.Delete(sessionID)
}

func (r *Router) Start() {
    r.ticker = time.NewTicker(AggregateInterval)
    go r.run()
}

func (r *Router) Stop() {
    r.ticker.Stop()
    close(r.stop)
}

func (r *Router) run() {
    for {
        select {
        case <-r.ticker.C:
            r.aggregate()
        case <-r.stop:
            return
        }
    }
}

func (r *Router) aggregate() {
    frame := &codec.Frame{
        Type:         codec.FrameTypeTerminalOutput,
        SessionCount: 0,
        Sessions:     make([]codec.SessionBlock, 0),
    }

    r.buffers.Range(func(key, value any) bool {
        rb := value.(*RingBuffer)
        sessionID := key.(uint32)

        data := r.pool.Get(4096)
        defer r.pool.Put(data)

        n, _ := rb.Read(data)
        if n > 0 {
            frame.Sessions = append(frame.Sessions, codec.SessionBlock{
                SessionID: sessionID,
                Length:    uint16(n),
                Data:      data[:n],
            })
            frame.SessionCount++
        }
        return true
    })

    if frame.SessionCount > 0 {
        serialized := frame.Serialize()
        r.hub.Broadcast(serialized)
    }
}
```

- [ ] **Step 2: 添加 Hub.Broadcast**

```go
func (h *Hub) Broadcast(data []byte) {
    h.ConnMap.Range(func_, value any) bool {
        conn := value.(*Conn)
        select {
        case conn.Send <- data:
        default:
        }
        return true
    })
}
```

- [ ] **Step 3: 提交**

```bash
git add gateway/internal/router/ gateway/internal/server/
git commit -m "feat(gateway): add Stream Router with 20ms aggregation"
```

---

### 任务 3.3: Backpressure 流控机制

**Files:**
- Modify: `gateway/internal/router/router.go`
- Modify: `gateway/internal/session/session.go`

- [ ] **Step 1: 添加 Backpressure 逻辑**

```go
// 在 router.go 的 aggregate 中添加

r.buffers.Range(func(key, value any) bool {
    rb := value.(*RingBuffer)
    sessionID := key.(uint32)

    s, ok := r.sessionMgr.Get(sessionID)
    if !ok {
        return true
    }

    // 检查水位
    if rb.IsHigh() && !s.IsSlow {
        s.IsSlow = true
        // 发送 SlowWarning 到客户端
        r.sendSlowWarning(sessionID, true)
    } else if rb.IsLow() && s.IsSlow {
        s.IsSlow = false
        // 恢复读取
        r.sendSlowWarning(sessionID, false)
    }

    // 如果是慢节点，跳过读取
    if s.IsSlow {
        return true
    }

    data := r.pool.Get(4096)
    defer r.pool.Put(data)

    n, _ := rb.Read(data)
    if n > 0 {
        frame.Sessions = append(frame.Sessions, codec.SessionBlock{
            SessionID: sessionID,
            Length:    uint16(n),
            Data:      data[:n],
        })
        frame.SessionCount++
    }
    return true
})
```

- [ ] **Step 2: 添加 sendSlowWarning**

```go
func (r *Router) sendSlowWarning(sessionID uint32, isSlow bool) {
    warningType := []byte{0x06} // SlowWarning 帧类型
    sessionIDBytes, _ := session.MarshalText()
    data := []byte(fmt.Sprintf(`{"type":"slow","sessionId":%d,"isSlow":%v}`, sessionID, isSlow))

    frame := &codec.Frame{
        Type:         0x06,
        SessionCount: 1,
        Sessions: []codec.SessionBlock{
            {
                SessionID: sessionID,
                Length:    uint16(len(data)),
                Data:      data,
            },
        },
    }
    r.hub.Broadcast(frame.Serialize())
}
```

- [ ] **Step 3: 在 Session 添加 IsSlow 字段访问器**

```go
func (s *Session) SetSlow(slow bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.IsSlow = slow
}
```

- [ ] **Step 4: 提交**

```bash
git add gateway/internal/router/ gateway/internal/session/
git commit -m "feat(gateway): add backpressure flow control with slow node detection"
```

---

### 任务 3.4: 前端乐观回显与渲染节流

**Files:**
- Modify: `frontend/src/components/Terminal.vue`
- Modify: `frontend/src/workers/ws.worker.ts`
- Create: `frontend/src/stores/terminal.ts`

- [ ] **Step 1: 创建 Pinia Store**

```typescript
// frontend/src/stores/terminal.ts
import { defineStore } from 'pinia';

interface TerminalState {
  sessions: Map<number, {
    isSlow: boolean;
    tuiState: boolean;
    echoLock: boolean;
  }>;
}

export const useTerminalStore = defineStore('terminal', {
  state: (): TerminalState => ({
    sessions: new Map(),
  }),
  actions: {
    setSlow(sessionId: number, isSlow: boolean) {
      const session = this.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
      session.isSlow = isSlow;
      this.sessions.set(sessionId, session);
    },
    setEchoLock(sessionId: number, locked: boolean) {
      const session = this.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
      session.echoLock = locked;
      this.sessions.set(sessionId, session);
    },
    getEchoLock(sessionId: number) {
      return this.sessions.get(sessionId)?.echoLock ?? false;
    },
  },
});
```

- [ ] **Step 2: 添加乐观回显**

```typescript
// 在 ws.worker.ts 中

let echoLockSessions = new Set<number>();
let pendingEchoFrames: Map<number, Uint8Array[]> = new Map();

function handleInput(frame: ParsedFrame) {
  // 设置 EchoLock
  for (const block of frame.sessions) {
    echoLockSessions.add(block.sessionId);
    pendingEchoFrames.set(block.sessionId, []);
  }

  // 50ms 后释放锁
  setTimeout(() => {
    for (const block of frame.sessions) {
      echoLockSessions.delete(block.sessionId);
      // 处理积压的帧
      const pending = pendingEchoFrames.get(block.sessionId) || [];
      for (const data of pending) {
        self.postMessage({ type: 'output', sessionId: block.sessionId, data });
      }
      pendingEchoFrames.delete(block.sessionId);
    }
  }, 50);
}

self.onmessage = (e: MessageEvent<WorkerMessage>) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'frame':
      const frame = payload as ParsedFrame;
      if (frame.type === FrameType.TerminalInput) {
        handleInput(frame);
      }
      // ... 其他处理
      break;
  }
};
```

- [ ] **Step 3: 添加 rAF 节流**

```typescript
// 在 Terminal.vue 中

let outputQueue: string[] = [];
let rafId: number | null = null;

function scheduleFlush() {
  if (rafId !== null) return;
  rafId = requestAnimationFrame(() => {
    if (outputQueue.length > 0) {
      terminal?.write(outputQueue.join(''));
      outputQueue = [];
    }
    rafId = null;
  });
}

function queueOutput(data: string) {
  outputQueue.push(data);
  scheduleFlush();
}

// 修改 writeToTerminal
function writeToTerminal(data: string) {
  queueOutput(data);
}
```

- [ ] **Step 4: 提交**

```bash
git add frontend/src/
git commit -m "feat(frontend): add optimistic echo lock and rAF throttling"
```

---

## Phase 3 完成检查清单

- [ ] RingBuffer 写入/读取正常
- [ ] 20ms 聚合下发工作
- [ ] 85% 水位标记慢节点
- [ ] 50% 水位恢复
- [ ] 前端 EchoLock 50ms 窗口
- [ ] rAF 节流渲染正常

---

## Phase 4: 批量广播 (Multiexec) 与一致性保障

### 任务 4.1: 前端多终端布局与状态同步

**Files:**
- Create: `frontend/src/stores/client.ts`
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: 创建 Client Store**

```typescript
// frontend/src/stores/client.ts
import { defineStore } from 'pinia';

interface ClientState {
  activeSessionId: number | null;
  selectedSessionIds: number[];
  mode: 'single' | 'multi';
}

export const useClientStore = defineStore('client', {
  state: (): ClientState => ({
    activeSessionId: null,
    selectedSessionIds: [],
    mode: 'single',
  }),
  actions: {
    setActive(sessionId: number) {
      this.activeSessionId = sessionId;
    },
    toggleSelect(sessionId: number) {
      const idx = this.selectedSessionIds.indexOf(sessionId);
      if (idx >= 0) {
        this.selectedSessionIds.splice(idx, 1);
      } else {
        this.selectedSessionIds.push(sessionId);
      }
      this.mode = this.selectedSessionIds.length > 1 ? 'multi' : 'single';
    },
    selectOnly(sessionId: number) {
      this.selectedSessionIds = [sessionId];
      this.activeSessionId = sessionId;
      this.mode = 'single';
    },
    clearSelection() {
      this.selectedSessionIds = [];
      this.mode = 'single';
    },
  },
});
```

- [ ] **Step 2: 更新 App.vue**

```vue
<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="toolbar">
      <span>模式: {{ mode }}</span>
      <span>已选: {{ selectedIds.join(', ') || '无' }}</span>
    </div>
    <div class="terminal-grid">
      <div
        v-for="session in sessions"
        :key="session.id"
        :class="['terminal-wrapper', { active: session.id === activeId, selected: selectedIds.includes(session.id), slow: session.isSlow }]"
        @click="handleClick(session.id)"
        @click.shift="handleShiftClick(session.id)"
      >
        <Terminal :sessionId="session.id" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useClientStore } from './stores/client';
import Terminal from './components/Terminal.vue';

const clientStore = useClientStore();
const activeId = computed(() => clientStore.activeSessionId);
const selectedIds = computed(() => clientStore.selectedSessionIds);
const mode = computed(() => clientStore.mode);

const sessions = [
  { id: 1, isSlow: false },
  { id: 2, isSlow: false },
  { id: 3, isSlow: false },
];

function handleClick(id: number) {
  clientStore.selectOnly(id);
}

function handleShiftClick(id: number) {
  clientStore.toggleSelect(id);
}
</script>

<style scoped>
.toolbar {
  padding: 10px;
  background: #2d2d2d;
  color: #d4d4d4;
  margin-bottom: 10px;
}
.terminal-wrapper {
  border: 2px solid transparent;
}
.terminal-wrapper.active {
  border-color: #0066cc;
}
.terminal-wrapper.selected {
  border-color: #00cc66;
}
.terminal-wrapper.slow {
  background: rgba(255, 200, 0, 0.2);
}
</style>
```

- [ ] **Step 3: 提交**

```bash
git add frontend/src/
git commit -m "feat(frontend): add multi-terminal selection UI"
```

---

### 任务 4.2: Multiexec Engine 广播逻辑

**Files:**
- Create: `gateway/internal/multiexec/engine.go`

- [ ] **Step 1: 创建 Multiexec Engine**

```go
package multiexec

import (
    "fmt"
    "sync"

    "github.com/mensylisir/multi-terminal/gateway/internal/session"
)

type Engine struct {
    sessionMgr *session.Manager
    queues     sync.Map
}

func NewEngine(sessionMgr *session.Manager) *Engine {
    return &Engine{
        sessionMgr: sessionMgr,
    }
}

func (e *Engine) Broadcast(input []byte, sourceSessionID uint32, targetSessionIDs []uint32) error {
    errors := make([]error, 0)

    for _, targetID := range targetSessionIDs {
        if targetID == sourceSessionID {
            continue // 不向源 session 广播
        }

        s, ok := e.sessionMgr.Get(targetID)
        if !ok {
            errors = append(errors, fmt.Errorf("session %d not found", targetID))
            continue
        }

        if s.GetState() != session.StateActive {
            errors = append(errors, fmt.Errorf("session %d not active", targetID))
            continue
        }

        // 异步写入目标 session
        go func(sess *session.Session, data []byte) {
            _, err := sess.ShellStream.Write(data)
            if err != nil {
                // 记录错误，可能需要通知前端
            }
        }(s, input)
    }

    if len(errors) > 0 {
        return fmt.Errorf("broadcast errors: %v", errors)
    }
    return nil
}
```

- [ ] **Step 2: 提交**

```bash
git add gateway/internal/multiexec/
git commit -m "feat(gateway): add Multiexec broadcast engine"
```

---

### 任务 4.3: 上下文状态探测 (Context Checker)

**Files:**
- Create: `gateway/internal/session/context_checker.go`

- [ ] **Step 1: 创建 Context Checker**

```go
package session

import (
    "bytes"
    "regexp"
    "time"
)

var (
    shellPromptRegex = regexp.MustCompile(`[\$#]\s*$`)
    tuiEscapeRegex   = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

const (
    ProbeTimeout = 2 * time.Second
)

type ProbeResult struct {
    SessionID    uint32
    PromptState  PromptState
    RawResponse  []byte
}

func (mgr *Manager) ProbeSession(sessionID uint32) (*ProbeResult, error) {
    s, ok := mgr.Get(sessionID)
    if !ok {
        return nil, fmt.Errorf("session not found")
    }

    // 发送探针
    probe := []byte{0x05} // ENQ
    _, err := s.ShellStream.Write(probe)
    if err != nil {
        return nil, err
    }

    // 等待响应
    buf := make([]byte, 1024)
    result := &ProbeResult{SessionID: sessionID}

    deadline := time.Now().Add(ProbeTimeout)
    for time.Now().Before(deadline) {
        s.ShellStream.SetReadDeadline(deadline)
        n, err := s.ShellStream.Read(buf)
        if err != nil {
            if err == io.EOF || os.IsTimeout(err) {
                result.PromptState = PromptUnknown
                return result, nil
            }
            return nil, err
        }
        result.RawResponse = append(result.RawResponse, buf[:n]...)

        // 分析响应
        if shellPromptRegex.Match(result.RawResponse) {
            result.PromptState = PromptShell
            return result, nil
        }
        if tuiEscapeRegex.Match(result.RawResponse) {
            result.PromptState = PromptTUI
            return result, nil
        }
    }

    result.PromptState = PromptUnknown
    return result, nil
}
```

- [ ] **Step 2: 修复 import**

```go
import (
    "bytes"
    "fmt"
    "io"
    "os"
    "regexp"
    "time"
)
```

- [ ] **Step 3: 提交**

```bash
git add gateway/internal/session/
git commit -m "feat(gateway): add context checker for shell/tui detection"
```

---

### 任务 4.4: 终端回显锁定器

回显锁定器已在 Phase 3.4 (ws.worker.ts) 实现。

- [ ] **Step 1: 验证实现**

检查 `frontend/src/workers/ws.worker.ts` 中的 EchoLock 逻辑是否完整。

- [ ] **Step 2: 提交 (如需更新)**

```bash
git add frontend/src/workers/
git commit -m "feat(frontend): verify echo lock implementation"
```

---

## Phase 4 完成检查清单

- [ ] Pinia Client Store 状态同步正常
- [ ] Shift+Click 多选功能
- [ ] Multiexec Engine 广播工作
- [ ] Context Checker 探针发送与识别
- [ ] Shell/TUI/Unknown 状态识别
- [ ] EchoLock 50ms 窗口工作

---

## Phase 5-10 任务列表

由于篇幅限制，Phase 5-10 的详细任务将单独创建执行计划。

**Phase 5: 数据隔离审计与高可用**
- Diff Engine (ANSI Strip + Hash)
- Audit Service (Zstd 压缩 + S3)
- 断线重连 (detached 恢复)
- Graceful Restart (FD handoff)

**Phase 6: 测试与持续集成**
- 单元测试 + 集成测试
- E2E 测试
- Dockerfile + CI/CD

**Phase 7: 可观测性与监控告警**
- Prometheus Metrics
- Grafana Dashboard

**Phase 8: 安全风控与命令拦截**
- 规则引擎
- Pre-Execution Check
- 前端二次确认 Modal

**Phase 9: 前端性能体验极致优化**
- WebGL fallback 完整
- 精确 Selector 订阅
- rAF 批量渲染

**Phase 10: 全局资源守卫与限流防护**
- ulimit 预检
- FD 监控断路器
- Rate Limiting

---

## 执行策略

**推荐方式**: Subagent-Driven Development
- 每个 Phase 由独立 subagent 执行
- 阶段完成后进行代码审查
- 确保 24 小时不间断开发

**下一步**: 开始 Phase 1 执行
