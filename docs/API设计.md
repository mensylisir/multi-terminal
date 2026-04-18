# API设计

本文档详细描述了系统中的核心 API 接口与数据交互协议。在此系统中，所有的输入输出（包括终端交互、控制命令、文件传输等）均承载于高频低延迟的连接之上。

## 1. WebSocket 协议与连接生命周期

系统核心通信采用 WebSocket 二进制帧实现多路复用，这要求所有的 API 交互基于帧（Frame）的设计理念。

### 1.1 连接建立与鉴权
- **握手阶段**: 客户端发起 WSS (WebSocket Secure) 连接请求，附带用户认证信息（Token）。通常可以在 URL Query 中携带，例如 `wss://gateway.example.com/ws?token=eyJhbG...`。
- **初始化阶段 (Auth & Config)**:
  - 建立连接后，网关首先校验 Token。
  - 若鉴权通过，服务端下发一条全局的初始化配置项（作为二进制中的一条特殊的 Session Control 控制帧，或初始 JSON 负载）。

  *初始化下发数据结构示例：*
  ```json
  {
    "type": "INIT_ACK",
    "data": {
      "userId": "admin_001",
      "quotas": {
        "maxSessions": 20,
        "maxMultiexec": 10
      },
      "config": {
        "heartbeatIntervalMs": 10000,
        "riskLevel": "high"
      }
    }
  }
  ```
  - 若鉴权失败，服务端立即返回状态码 401 或特定的错误帧，然后关闭 WebSocket。
- **心跳保活**: 客户端需每隔固定时间发送心跳包（Ping），服务端响应 Pong，长时间无响应将触发连接断开与会话挂起流程。

## 2. 二进制帧结构详解

整个系统统一定义使用 **Big-Endian（网络字节序）**，以确保跨平台解析的一致性。

### 2.1 基础帧结构

为了兼顾灵活性与极小的开销，我们的帧结构被设计为紧凑的定长首部加上可变长的负载（Payload）。

```text
+---------------+------------------+
| FrameType (1B)| SessionCount(1B) |
+---------------+------------------+
|                                  |
|  [ Session Block 1 ]             |
|  [ Session Block 2 ]             |
|  ...                             |
|  [ Session Block N ]             |
|                                  |
+----------------------------------+

[ Session Block 结构 ]
+-------------------+----------------+---------------------------+
| SessionId (4B)    | Length (2B)    | Data (Length Bytes)       |
+-------------------+----------------+---------------------------+
```

#### 字段详细说明：
- **FrameType (1 Byte)**: 表示帧的指令类型，定义了接收端应对此帧采取的动作。
  - `0x01` - 心跳 (Heartbeat): 用于保持连接活跃，防止负载均衡器或 NAT 超时切断连接。通常 Data 块可以为空（长度 0）。
  - `0x02` - 终端输出 (Terminal Output): Gateway 向浏览器发送的远端 PTY 的标准输出流。
    - **Data Payload**: 纯粹的终端字节流，可能包含 ANSI 控制转义序列，前端解包后直接送给 `xterm.js` 的 `write()` 方法处理。
  - `0x03` - 终端输入 (Terminal Input): 浏览器向 Gateway 发送的用户键盘或鼠标交互指令。
    - **Data Payload**: UTF-8 编码的用户输入字符，或者特殊的终端控制字符（如 `0x03` 代表 Ctrl+C）。
  - `0x04` - 窗口调整 (Window Resize): 浏览器向后端发送，携带终端重置大小事件。
    - **Data Payload**: 固定长度 4 Bytes。前 2 字节为 `Cols` (uint16)，后 2 字节为 `Rows` (uint16)。例如：`[0x00, 0x50, 0x00, 0x18]` 代表 80列 24行。
  - `0x05` - 会话控制 (Session Control): 用于前端发送新建连接或主动销毁连接的信号。
    - **Data Payload**: UTF-8 编码的 JSON 字符串，例如 `{"action": "create", "hostId": "web-01"}`。
- **SessionCount (1 Byte)**: 此帧包含的 Session 数据块数量，支持单帧携带多会话数据（最大 255）。
- **Session 数据块 (循环 SessionCount 次)**:
  - **SessionId (4 Bytes)**: `uint32` 类型，唯一标识一个终端会话。
  - **Length (2 Bytes)**: `uint16` 类型，随后的数据负载长度 (Data 的字节数)。最大支持 65535 字节。
  - **Data (N Bytes)**: 实际的负载内容，如终端的 ANSI 字符流、Resize 的 cols/rows 参数等。

### 2.2 前端解析与性能要求
由于采用了紧凑的二进制结构，前端处理时有极高的性能规范要求：
- 必须使用 `DataView` 和 `Uint8Array` 进行内存级别的解析与位运算。
- 必须严格按照字节偏移（Offset）读取数据。
- 明确禁止使用 JSON fallback，不允许将二进制流转为字符串后再通过 JSON 解析，以保证执行效率和极低延迟。

## 3. 聚合发送机制 (Stream Router)

为了解决多开 Session 导致的海量碎包问题：
- 后端采用独立的 **Stream Router** 组件。
- **20ms 窗口聚合**: 服务端维护一个时间窗口为 20ms 的定时器，在窗口期内聚合所有 active session 产生的输出流。
- **多路复用封装**: 聚合后的多路数据将被打包成单一的 WebSocket 二进制帧，一次性下发到前端。这极大地降低了网络 I/O 频次，避免 TCP 粘包拆包带来的逻辑开销。

## 4. 边界情况与错误处理

为了保证在高并发下二进制解析引擎的绝对稳定，系统必须实现以下边界防御：

- **数据截断与粘包防御**: 在 WebSocket 接收循环中，可能因网络 MTU 限制导致逻辑帧被切割。
  - 前端 Worker / 后端解析器在读取 Header (`FrameType` + `SessionCount`) 后，必须根据 `Length` 字段预先检查 `Uint8Array` 剩余字节数。
  - 若 `Length` 标明的值大于实际剩余可读长度，说明发生了**数据截断**。此时必须将不完整的 Buffer 暂存，等待下一个 WS 事件的碎片数据拼接后再行解析，绝不可抛出 `Index Out of Bounds` 异常导致进程/Worker 崩溃。
- **未知 FrameType 降级**: 随着系统迭代，可能引入新的控制帧。遇到未知的 `FrameType` 时，解析器应利用首部中明确的 `Length` 字段直接将读取游标 (Offset) 后移以跳过该数据块，并记录一条 Warn 级别的日志，绝不能抛出异常或中断整个长连接，从而保障前后端多版本共存时的向前兼容性。
- **慢节点限流响应**: 当服务端触发针对某 session 的流控时，会通过特定的控制帧（Slow Warning）告知前端，前端在 UI 上展示黄色的警告状态，告知用户该节点正在遭遇反压。
