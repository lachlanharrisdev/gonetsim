ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download

COPY . ./

ARG VERSION=dev
ARG REVISION=dev
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 \
    go build -trimpath \
      -ldflags "-s -w -X github.com/lachlanharrisdev/gonetsim/cmd.Version=${VERSION} -X github.com/lachlanharrisdev/gonetsim/cmd.Revision=${REVISION} -X github.com/lachlanharrisdev/gonetsim/cmd.Date=${BUILD_DATE}" \
      -o /out/gonetsim \
      .


FROM alpine:3.22

RUN apk add --no-cache ca-certificates su-exec && update-ca-certificates

RUN addgroup -S gonetsim \
  && adduser -S -G gonetsim -h /home/gonetsim -s /sbin/nologin gonetsim \
  && mkdir -p /etc/gonetsim /work /usr/local/share/ca-certificates \
  && chown -R gonetsim:gonetsim /etc/gonetsim /work

# prefers /etc/gonetsim/gonetsim.toml if present
# including default config keeps the container runnable with no mounts
COPY --from=builder --chown=gonetsim:gonetsim /src/internal/config/default_config.toml /etc/gonetsim/gonetsim.toml

COPY --from=builder /out/gonetsim /gonetsim
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod 0755 /gonetsim /entrypoint.sh

WORKDIR /work

EXPOSE 5353/udp
EXPOSE 5353/tcp
EXPOSE 8080/tcp
EXPOSE 8443/tcp
EXPOSE 1025/tcp
EXPOSE 1465/tcp

ENTRYPOINT ["/entrypoint.sh"]
