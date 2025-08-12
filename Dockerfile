# 多阶段构建 - 优化镜像大小
FROM golang:alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 先复制依赖文件，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 静态编译，减少依赖
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o main ./cmd/radar

# 运行阶段 - 使用 distroless 镜像
FROM gcr.io/distroless/static-debian11:nonroot

# 复制必要文件
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/main /app/main

WORKDIR /app
EXPOSE 8080

ENTRYPOINT ["/app/main"]
