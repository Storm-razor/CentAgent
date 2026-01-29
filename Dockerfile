# CentAgent 镜像构建说明：
# - 使用多阶段构建：builder 负责编译 Go 二进制；runtime 保持镜像精简且自带 shell，方便 docker exec 进入容器。
# - 运行时默认执行 `centagent start`，以便容器启动后自动进入“监控常驻”模式。
# - 配置文件与数据库建议通过 volume/bind mount 动态挂载（见 docker-compose.yml）。

ARG GO_VERSION=1.24
ARG REGISTRY=docker.io
ARG ALPINE_MIRROR=https://dl-cdn.alpinelinux.org/alpine
ARG GOPROXY=https://goproxy.cn,direct

FROM ${REGISTRY}/library/golang:${GO_VERSION}-alpine AS builder


ARG ALPINE_MIRROR
ARG GOPROXY
ARG REGISTRY

WORKDIR /src

RUN if [ -n "${ALPINE_MIRROR}" ]; then \
      printf '%s\n' \
        "${ALPINE_MIRROR}/v3.20/main" \
        "${ALPINE_MIRROR}/v3.20/community" \
        > /etc/apk/repositories; \
    fi

ENV GOPROXY=${GOPROXY}

# CA 用于 go mod 下载走 HTTPS（使用 GOPROXY 时通常不需要系统 git）
RUN apk add --no-cache ca-certificates

# 先拷贝 go.mod/go.sum 以利用 Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# 再拷贝完整源码并编译
COPY . .

# 支持 buildx 透传的 TARGETOS/TARGETARCH；未提供时使用 linux/amd64
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# 该项目使用 modernc sqlite（纯 Go），因此 CGO 可禁用，得到更易部署的二进制
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/centagent ./cmd/centagent


FROM ${REGISTRY}/library/alpine:3.20 AS runtime

ARG ALPINE_MIRROR=https://dl-cdn.alpinelinux.org/alpine
RUN if [ -n "${ALPINE_MIRROR}" ]; then \
      printf '%s\n' \
        "${ALPINE_MIRROR}/v3.20/main" \
        "${ALPINE_MIRROR}/v3.20/community" \
        > /etc/apk/repositories; \
    fi

# 运行时需要 CA（访问 Ark/OpenAI 类 HTTPS API）；保留 sh 便于进入容器排障/执行命令
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/centagent /usr/local/bin/centagent

# 提供一个“可用的默认配置模板”，方便不挂载时也能启动（但仍需提供 ARK_* 环境变量）
RUN mkdir -p /etc/centagent /var/lib/centagent /app/configs
COPY ./configs/config.yaml /etc/centagent/config.yaml
COPY ./configs/config.yaml /app/configs/config.yaml

# 默认把数据库放到可持久化目录；你也可以在 config.yaml 或环境变量覆盖它
ENV CENTAGENT_STORAGE_PATH=/var/lib/centagent/centagent.db

# 容器默认常驻运行监控；用户可通过 `docker exec` 进入执行 chat/storage 等命令
ENTRYPOINT ["centagent"]
CMD ["start", "--config", "/etc/centagent/config.yaml"]
