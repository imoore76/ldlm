FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.22 as builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN mkdir /dist
WORKDIR /dist/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o ldlm-server ./cmd/server/
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o ldlm-lock ./cmd/lock/

FROM --platform=${TARGETPLATFORM:-linux/amd64} debian:bookworm-slim
COPY --from=builder /dist/ldlm-server /usr/bin/ldlm-server
COPY --from=builder /dist/ldlm-lock /usr/bin/ldlm-lock

LABEL org.opencontainers.image.source=https://github.com/imoore76/go-ldlm
LABEL org.opencontainers.image.description="LDLM server"
LABEL org.opencontainers.image.licenses="Apache 2.0"

ENV LDLM_LISTEN_ADDRESS="0.0.0.0:3144" LDLM_IPC_SOCKET_FILE="/tmp/ldlm-ipc.sock"
EXPOSE "3144/tcp"

RUN set -eux; \
        groupadd -r ldlm --gid=999; \
        useradd -r -g ldlm --uid=999 --home-dir=/home/ldlm --shell=/bin/bash ldlm; \
        mkdir -p /home/ldlm; chown -R ldlm:ldlm /home/ldlm

WORKDIR /home/ldlm

USER ldlm:ldlm

CMD ["/usr/bin/ldlm-server"] 
