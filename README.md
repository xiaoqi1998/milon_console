# Milon API Server

将 Milon Go SDK 封装为 RESTful HTTP 接口，并提供 Web 调试控制台。

## 功能特性

- **网络切换**：支持 localNet / devNet 一键切换
- **32 个 RESTful API 端点**：覆盖系统/账户/交易/合约/RPC/工具/领水/视图八大分类
- **Web 控制台**：深色主题调试控制台，支持端点导航、参数编辑、响应展示、请求历史
- **Gas 费用透传**：模拟与交易回执中均返回 `gasCharged` 字段，前端高亮展示
- **4 种支付模式**：`unified_payer_all` / `unified_dual_sign` / `unified_payer_only_gas` / `split`
- **多密钥类型**：账户生成支持 secp256k1 / ed25519 / bls12381 / fndsa512
- **领水功能**：内置 faucet 领水与余额查询，领水成功返回 `txHash` 可追踪
- **工具接口**：地址派生、公钥派生、签名、验签

---

## 本地电脑使用

### 方式一：直接运行（推荐开发）

#### 环境要求

- Go 1.25+
- CGO 启用（因 SDK 依赖 blst 库，需要 C 编译器）
  - **Windows**：安装 MinGW-w64
    1. 下载：https://winlibs.com/ （选 Win64 GNU/UCRT runtime）
    2. 解压后将 mingw64/bin 加入系统 PATH
    3. 验证：gcc --version
  - **Linux**：sudo apt install gcc
  - **macOS**：xcode-select --install

#### 步骤

```bash
# 1. 克隆项目（SDK 已内置在 gosdk-develop/ 目录，无需额外下载）
git clone https://github.com/xiaoqi1998/milon_console.git
cd milon_console

# 2. 启用 CGO
go env -w CGO_ENABLED=1

# 3. 下载依赖
go mod tidy

# 4. 编译运行
go run main.go

# 或编译成可执行文件
go build -o milon-api-server .
./milon-api-server
```
访问 http://localhost:8080 打开 Web 控制台。

### 方式二：本地 Docker 运行

#### 环境要求

- Docker Desktop（Windows/macOS）或 Docker Engine（Linux）
- 至少 2GB 内存

#### 步骤

```bash
# 构建并启动（build context 为项目根目录，SDK 已内置）
docker compose build
docker compose up -d

# 查看日志
docker compose logs -f

# 停止服务
docker compose down
```
访问 http://localhost:8080。

---
## 服务器 Docker 部署

### 前置准备

1. **服务器要求**：
   - Linux 系统（推荐 Ubuntu 22.04+ / Debian 12+）
   - 至少 2 核 CPU、2GB 内存
   - Docker 和 Docker Compose 已安装

2. **安装 Docker**（如果尚未安装）：
```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
# 重新登录后生效
```
3. **上传项目文件**：
   - 将 milon_console 目录上传到服务器（SDK 已内置在 `gosdk-develop/` 子目录，无需单独上传）
   - 推荐使用 scp 或 rsync

### 目录结构

```
/opt/milon/
└── milon_console/          # 本项目（SDK 内置在 gosdk-develop/ 子目录）
    ├── gosdk-develop/      # Milon Go SDK 源码（内置）
    ├── Dockerfile
    ├── docker-compose.yml
    ├── .dockerignore
    ├── main.go
    ├── go.mod
    ├── go.sum
    └── ...
```
### 部署步骤

```bash
# 1. 进入项目目录
cd /opt/milon/milon_console

# 2. 构建镜像
docker compose build

# 3. 启动服务（后台运行）
docker compose up -d

# 4. 查看是否启动成功
docker compose ps
docker compose logs -f

# 5. 测试接口
curl http://localhost:8080/api/health
```
### 配置自定义

修改 docker-compose.yml 中的环境变量，或创建 .env 文件：

```env
# 端口
SERVER_PORT=8080

# 默认网络
DEFAULT_NETWORK=devNet

# CORS 允许来源（生产环境建议指定域名）
ALLOWED_ORIGINS=https://your-domain.com

# 是否启用签名工具接口（生产环境建议关闭）
ENABLE_UTIL_SIGN=false

# 服务端签名私钥（可选，用于 write 接口）
# SIGNER_PRIVATE_KEY=base58_or_hex_private_key

# 自定义 RPC 地址（可选）
# MILON_RPC_URL=http://your-node:6280/milon/v1
```
修改后重启服务：
```bash
docker compose up -d --force-recreate
```
### Nginx 反向代理（推荐生产环境）

```nginx
server {
    listen 80;
    server_name api.your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
配置 HTTPS 建议使用 Let's Encrypt + Certbot。

### 常用运维命令

```bash
# 查看状态
docker compose ps

# 查看日志
docker compose logs -f --tail=100

# 重启服务
docker compose restart

# 停止服务
docker compose down

# 更新代码后重新构建
git pull
docker compose build
docker compose up -d

# 清理旧镜像
docker image prune -f
```
---
## 配置说明

通过环境变量配置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| SERVER_PORT | 8080 | 服务端口 |
| ALLOWED_ORIGINS | * | CORS 允许来源 |
| ENABLE_UTIL_SIGN | false | 是否启用签名工具接口 |
| SIGNER_PRIVATE_KEY | (空) | 服务端签名私钥 |
| DEFAULT_NETWORK | devNet | 默认网络 |
| MILON_RPC_URL | (内置) | 自定义 Milon 链节点 RPC 地址 |

---

## API 端点（共 32 个）

### 网络管理（3）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/network/list | 获取网络列表 |
| GET | /api/network/current | 获取当前网络 |
| POST | /api/network/switch | 切换网络 |

### 系统（2）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/health | 健康检查（返回 blockHeight、chainId） |
| GET | /api/chain-head | 获取链头 |

### 账户（3）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/accounts/:address | 获取账户信息 |
| GET | /api/accounts/:address/resources | 获取该账户的资源列表 |
| POST | /api/accounts/generate | 生成账户（支持 keyType: secp256k1/ed25519/bls12381/fndsa512） |

### 交易（6）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/transactions/:hash | 按哈希查交易（回执含 gasCharged） |
| GET | /api/transactions/:hash/events | 获取交易事件 |
| GET | /api/transactions/:hash/wait | 等待交易确认（回执含 gasCharged） |
| POST | /api/transactions/simulate | 底层模拟（返回 gasCharged） |
| POST | /api/transactions/submit | 底层提交（base64 postcard） |
| POST | /api/transactions/inspect | 检测交易（解析 postcard 返回 txHash/ixHashes/payer/valid） |

### 合约（7）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/read | 读取视图函数（单返回值） |
| POST | /api/read/multi | 读取视图函数（多返回值，封装 BuildAndViewMultiIx） |
| POST | /api/simulate | 模拟合约调用（4 种支付模式，返回 gasCharged） |
| POST | /api/write | 写入交易（4 种支付模式） |
| POST | /api/write/multi-agent | 多方签名写入（unified_dual_sign） |
| POST | /api/write/multisig | 多签写入（split） |
| POST | /api/view/single | 底层单指令视图（预构建 postcard） |
| POST | /api/view/multi | 底层多指令视图（预构建 postcard） |

### RPC（4）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/rpc/blocks/:height | 获取区块 |
| GET | /api/rpc/resources/:hash | 获取资源 |
| POST | /api/rpc/access-value | 获取访问值 |
| GET | /api/rpc/resource-paths/:hash | 按哈希获取资源路径 |

### 领水（2）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/faucet/claim | 领水（返回 address/claimed/txHash） |
| GET | /api/faucet/balance/:address | 查询 MIL 余额 |

### 工具（4）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/util/address/derive | 从公钥派生地址 |
| POST | /api/util/key/derive-public | 从私钥派生公钥（4 种密钥类型） |
| POST | /api/util/sign | 签名消息（需 ENABLE_UTIL_SIGN=true） |
| POST | /api/util/verify | 验签 |

---

### 支付模式说明

`/api/simulate` 与 `/api/write` 通过请求体 `paymentMode` 字段选择签名与 gas 支付方式：

| 支付模式 | 说明 | 适用场景 |
|---------|------|---------|
| `unified_payer_all` | 付款人签署全部（payer + ix） | 单账户代付 |
| `unified_dual_sign` | 付款人付 gas + ix 签名者签名 | 多方协作 |
| `unified_payer_only_gas` | 付款人仅付 gas，不签 ix | gas 代付场景 |
| `split` | owner 自付 gas 并签名 | 独立账户 |

`signatureMode` 请求字段格式为 `{"type":"pubkey","publicKey":"<base58公钥>"}`。

### Gas 费用说明

SDK 更新后的 gas 机制下，所有模拟与交易回执接口均透传 `gasCharged` 字段：
- `POST /api/simulate`、`POST /api/transactions/simulate` — 模拟回执 `gasCharged`
- `GET /api/transactions/:hash`、`GET /api/transactions/:hash/wait` — 交易回执 `gasCharged`
- sponsored 交易（如 `ClaimFaucet`）的 `gasCharged=0`，gas 由赞助者支付

前端控制台在响应展示区以 ⛽ 横幅高亮显示 gas 费用。

---

## 项目结构

```
milon-api-server/
├── Dockerfile               # Docker 镜像构建
├── docker-compose.yml       # Docker Compose 编排
├── .dockerignore            # Docker 构建忽略
├── .gitignore
├── main.go                  # 入口文件（路由注册）
├── go.mod
├── go.sum
├── config/config.go         # 配置管理
├── client/network_manager.go # 网络管理与切换
├── handler/                 # API 处理器
│   ├── network.go           # 网络管理接口
│   ├── system_handler.go    # 系统接口
│   ├── account_handler.go   # 账户接口（含多密钥类型）
│   ├── transaction_handler.go # 交易接口（含 inspect、gasCharged 透传）
│   ├── contract.go          # 合约接口（read/simulate/write，4 种支付模式）
│   ├── faucet_handler.go    # 领水接口（claim 返回 txHash + balance 查询）
│   ├── view_handler.go      # 底层视图接口（single/multi）
│   ├── rpc_read.go          # RPC 底层接口
│   ├── resource_path_handler.go # 资源路径接口
│   └── util.go              # 工具接口（地址/公钥派生、签名、验签）
├── middleware/              # 中间件
│   ├── cors.go              # CORS
│   └── logging.go           # 请求日志
├── types/                   # 类型定义
│   ├── request.go           # 请求结构体
│   ├── response.go          # 响应结构体
│   └── conversion.go        # JSON 转换辅助
└── static/                  # 前端控制台
    ├── index.html
    ├── css/style.css
    └── js/app.js
```

---

## 技术栈

- **后端**：Go + Gin + Milon Go SDK
- **前端**：原生 HTML/CSS/JavaScript（无框架依赖）
- **部署**：Docker + Docker Compose
- **依赖**：gin-gonic/gin、gin-contrib/cors、milon-go-sdk
