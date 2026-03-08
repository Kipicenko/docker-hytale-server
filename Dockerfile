ARG GO_VERSION=1.26.1
ARG JAVA_VERSION=25
ARG ALPINE_VERSION=3.23

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /dist/server ./cmd/hytale-server/main.go && \
    go build -o /dist/healthcheck ./cmd/healthcheck/main.go


FROM alpine:${ALPINE_VERSION} AS hytale-downloader-cli

ARG HYTALE_CLI_URL="https://downloader.hytale.com/hytale-downloader.zip"
ARG TARGETARCH

RUN apk add --no-cache curl unzip

WORKDIR /cli

# Download and install Hytale Downloader CLI (amd64 only - Hytale doesn't provide ARM binary)
# But it will also work on arm64 thanks to Docker Emulation.

# Waiting for the Hytale Downloader CLI on arm64 :)
# mv hytale-downloader-linux-$TARGETARCH hytale-downloader
RUN curl -fsSL "${HYTALE_CLI_URL}" -o hytale-downloader-cli.zip && \
    unzip -q hytale-downloader-cli.zip && \
    rm hytale-downloader-cli.zip && \
    mv hytale-downloader-linux-amd64 hytale-downloader && \
    chmod +x hytale-downloader

FROM eclipse-temurin:${JAVA_VERSION}-jre-alpine-${ALPINE_VERSION} AS java-wrapper

RUN apk add --no-cache \
    libgcc \
    libstdc++ \
    gcompat \
    unzip \
    su-exec \
    && rm -rf /var/cache/apk/*


FROM java-wrapper AS main

ARG UID=1000
ARG GID=1000

RUN addgroup -g ${GID} hytale && \
    adduser -D -u ${UID} -G hytale -h /data hytale

WORKDIR /opt/hytale

EXPOSE 5520/udp

VOLUME ["/data"]

COPY --chmod=755 ./scripts/entrypoint.sh /opt/hytale/scripts/
COPY --from=builder --chmod=755 /dist/server /opt/hytale/bin/
COPY --from=builder --chmod=755 /dist/healthcheck /opt/hytale/bin/

COPY --from=hytale-downloader-cli --chmod=755 /cli/hytale-downloader /opt/hytale/cli/

# Saving the Hytale Downloader CLI version
RUN /opt/hytale/cli/hytale-downloader -version > /opt/hytale/cli/.version

HEALTHCHECK --interval=30s --timeout=10s --start-period=120s --retries=3 \
    CMD ["/opt/hytale/bin/healthcheck"]

ENTRYPOINT ["/opt/hytale/scripts/entrypoint.sh"]

STOPSIGNAL SIGTERM