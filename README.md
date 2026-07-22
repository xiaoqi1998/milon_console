# Milon API Server

将 Milon Go SDK 封装为 RESTful HTTP 接口，并提供 Web 调试控制台。

## 功能特性

- **网络切换**：支持 localNet / devNet 一键切换
- **27 个 RESTful API 端点**：覆盖系统/账户/交易/合约/RPC/工具六大分类
- **Web 控制台**：深色主题调试控制台，支持端点导航、参数编辑、响应展示、请求历史
- **多签名模式**：支持统付（unified）/分账（split）/多方签名（dual-sign）
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

`ash
# 1. 克隆项目
git clone https://github.com/xiaoqi1998/milon_console.git
cd milon_console

# 注意：SDK 需要在同级目录 ../gosdk-develop

# 2. 启用 CGO
go env -w CGO_ENABLED=1

# 3. 下载依赖
go mod tidy

# 4. 编译运行
go run main.go

# 或编译成可执行文件
go build -o milon-api-server .
./milon-api-server
``n
访问 http://localhost:8080 打开 Web 控制台。

### 方式二：本地 Docker 运行

#### 环境要求

- Docker Desktop（Windows/macOS）或 Docker Engine（Linux）
- 至少 2GB 内存

#### 步骤

`ash
# 1. 确保 SDK 在同级目录
# 目录结构：
#   parent/
#     milon_console/     (本项目)
#     gosdk-develop/    (Milon Go SDK)

# 2. 构建镜像
cd milon_console
docker compose build

# 3. 启动服务
docker compose up -d

# 4. 查看日志
docker compose logs -f

# 5. 停止服务
docker compose down
``n
访问 http://localhost:8080。

---
## 服务器 Docker 部署

### 前置准备

1. **服务器要求**：
   - Linux 系统（推荐 Ubuntu 22.04+ / Debian 12+）
   - 至少 2 核 CPU、2GB 内存
   - Docker 和 Docker Compose 已安装

2. **安装 Docker**（如果尚未安装）：
`ash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker 
# 重新登录后生效
``n
3. **上传项目文件**：
   - 将 milon_console 和 gosdk-develop 两个目录上传到服务器同一父目录下
   - 推荐使用 scp 或 rsync

### 目录结构

`
/opt/milon/
├── gosdk-develop/          # Milon Go SDK 源码
└── milon_console/          # 本项目
    ├── Dockerfile
    ├── docker-compose.yml
    ├── .dockerignore
    ├── main.go
    ├── go.mod
    ├── go.sum
    └── ...
``n
### 部署步骤

`ash
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
``n
### 配置自定义

修改 docker-compose.yml 中的环境变量，或创建 .env 文件：

`env
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
``n
修改后重启服务：
`ash
docker compose up -d --force-recreate
``n
### Nginx 反向代理（推荐生产环境）

`
ginx
server {
    listen 80;
    server_name api.your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade ;
        proxy_set_header Connection upgrade;
        proxy_set_header Host System.Management.Automation.Internal.Host.InternalHost;
        proxy_set_header X-Real-IP ;
        proxy_set_header X-Forwarded-For ;
        proxy_set_header X-Forwarded-Proto ;
    }
}
``n
配置 HTTPS 建议使用 Let's Encrypt + Certbot。

### 常用运维命令

`ash
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
``n
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

---
## 项目结构

`
milon-api-server/
├── Dockerfile               # Docker 镜像构建
├── docker-compose.yml       # Docker Compose 编排
├── .dockerignore            # Docker 构建忽略
├── .gitignore
├── main.go                  # 入口文件
├── config/config.go         # 配置管理
├── client/network_manager.go # 网络管理与切换
├── handler/                 # API 处理器
│   ├── network.go           # 网络管理接口
│   ├── system_handler.go    # 系统接口
│   ├── account_handler.go   # 账户接口
│   ├── transaction_handler.go # 交易接口
│   ├── contract.go          # 合约接口
│   ├── rpc_read.go          # RPC 底层接口
│   └── util.go              # 工具接口
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
`

---

## 技术栈

- **后端**：Go + Gin + Milon Go SDK
- **前端**：原生 HTML/CSS/JavaScript（无框架依赖）
- **部署**：Docker + Docker Compose
- **依赖**：gin-gonic/gin、gin-contrib/cors、milon-go-sdk
