# --- Build Stage ---
FROM golang:1.25-bookworm AS builder

# CGO_ENABLED=0 + -trimpath：可复现构建（同 commit + 同 Go 版本 + 同平台
# 产出哈希一致），供 /api/version 自报指纹与用户本地构建比对核验。
# 项目无 cgo 依赖（纯 Go）。
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

# 先复制 go.mod/go.sum 和 SDK 源码（利用层缓存）
COPY go.mod go.sum ./
COPY gosdk-develop ./gosdk-develop

RUN go mod download

# 复制项目源码（含 .git：Go 借此自动 VCS stamping，把 commit 写进二进制供
# /api/version 与页脚开源核验展示；.git 仅存在于 builder 阶段，不进最终镜像）
COPY . .

# commit/时间兜底注入（仅当构建上下文无 .git 时才有机会生效，如 CI 从
# 归档源码构建）：
#   docker build --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
#     --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) ...
# 有 .git 时 VCS stamping 优先，无需传参；均无时 /api/version 返回 "unknown"。
ARG GIT_COMMIT=
ARG BUILD_DATE=
RUN go build -trimpath -ldflags="-s -w -X milon-api-server/handler.buildGitCommit=${GIT_COMMIT} -X milon-api-server/handler.buildBuildDate=${BUILD_DATE}" -o milon-api-server .

# --- Runtime Stage ---
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /build/milon-api-server .
COPY --from=builder /build/static ./static

ENV SERVER_PORT=8080 \
    DEFAULT_NETWORK=devNet \
    ENABLE_UTIL_SIGN=false

EXPOSE 8080

CMD ["./milon-api-server"]
