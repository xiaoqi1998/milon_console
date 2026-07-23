# --- Build Stage ---
FROM golang:1.25-bookworm AS builder

ENV CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

# 先复制 go.mod/go.sum 和 SDK 源码（利用层缓存）
COPY go.mod go.sum ./
COPY gosdk-develop ./gosdk-develop

RUN go mod download

# 复制项目源码
COPY . .

RUN go build -ldflags="-s -w" -o milon-api-server .

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
