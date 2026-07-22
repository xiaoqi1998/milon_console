# Milon API Server

将 Milon Go SDK 封装为 RESTful HTTP 接口，并提供 Web 调试控制台。

## 功能特性

- **网络切换**：支持 localNet / devNet 一键切换
- **27 个 RESTful API 端点**：覆盖系统/账户/交易/合约/RPC/工具六大分类
- **Web 控制台**：深色主题调试控制台，支持端点导航、参数编辑、响应展示、请求历史
- **多签名模式**：支持统付（unified）/分账（split）/多方签名（dual-sign）
- **工具接口**：地址派生、公钥派生、签名、验签

## 快速开始

### 环境要求

- Go 1.25+
- CGO 启用（因 SDK 依赖 blst 库，需要 C 编译器）
  - Windows：安装 MinGW-w64 并将 `mingw64\bin` 加入 PATH
  - Linux：`sudo apt install gcc`
  - macOS：`xcode-select --install`

### 编译运行

```bash
# 设置 CGO
go env -w CGO_ENABLED=1

# 下载依赖
go mod tidy

# 编译
go build -o milon-api-server .

# 运行
./milon-api-server

# 或直接运行
go run main.go
```

访问 http://localhost:8080 打开 Web 控制台。

### 配置

通过环境变量配置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| SERVER_PORT | 8080 | 服务端口 |
| ALLOWED_ORIGINS | * | CORS 允许来源 |
| ENABLE_UTIL_SIGN | false | 是否启用签名工具接口 |
| SIGNER_PRIVATE_KEY | (空) | 服务端签名私钥 |
| DEFAULT_NETWORK | devNet | 默认网络 |

## API 端点

### 网络管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/network/list | 获取网络列表 |
| GET | /api/network/current | 获取当前网络 |
| POST | /api/network/switch | 切换网络 |

### 系统
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/health | 健康检查 |
| GET | /api/chain-head | 获取链头 |

### 账户
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/accounts/:address | 获取账户信息 |
| GET | /api/accounts/:address/resources | 获取账户资源 |
| POST | /api/accounts/generate | 生成账户 |

### 交易
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/transactions/:hash | 按哈希查交易 |
| GET | /api/transactions/:hash/events | 获取交易事件 |
| GET | /api/transactions/:hash/wait | 等待交易确认 |
| POST | /api/transactions/simulate | 底层模拟 |
| POST | /api/transactions/submit | 底层提交 |

### 合约
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/read | 读取视图函数 |
| POST | /api/simulate | 模拟合约调用 |
| POST | /api/write | 写入交易 |
| POST | /api/write/multi-agent | 多方签名写入 |
| POST | /api/write/multisig | 多签写入 |

### RPC
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/rpc/blocks/:height | 获取区块 |
| GET | /api/rpc/resources/:hash | 获取资源 |
| POST | /api/rpc/access-value | 获取访问值 |

### 工具
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/util/address/derive | 从公钥派生地址 |
| POST | /api/util/key/derive-public | 从私钥派生公钥 |
| POST | /api/util/sign | 签名消息 |
| POST | /api/util/verify | 验签 |

## 项目结构

```
milon-api-server/
├── main.go                    # 入口文件
├── config/config.go           # 配置管理
├── client/network_manager.go  # 网络管理与切换
├── handler/                   # API 处理器
│   ├── network.go             # 网络管理接口
│   ├── system_handler.go      # 系统接口
│   ├── account_handler.go     # 账户接口
│   ├── transaction_handler.go # 交易接口
│   ├── contract.go            # 合约接口
│   ├── rpc_read.go            # RPC 底层接口
│   └── util.go                # 工具接口
├── middleware/                # 中间件
│   ├── cors.go                # CORS
│   └── logging.go             # 请求日志
├── types/                     # 类型定义
│   ├── request.go             # 请求结构体
│   ├── response.go            # 响应结构体
│   └── conversion.go          # JSON 转换辅助
└── static/                    # 前端控制台
    ├── index.html
    ├── css/style.css
    └── js/app.js
```

## 技术栈

- **后端**：Go + Gin + Milon Go SDK
- **前端**：原生 HTML/CSS/JavaScript（无框架依赖）
- **依赖**：gin-gonic/gin、gin-contrib/cors、milon-go-sdk
