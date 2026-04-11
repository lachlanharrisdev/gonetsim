ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download

COPY . ./

ARG TARGETOS
ARG TARGETARCH

ARG VERSION=dev
ARG REVISION=dev
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X github.com/lachlanharrisdev/gonetsim/cmd.Version=${VERSION} -X github.com/lachlanharrisdev/gonetsim/cmd.Revision=${REVISION} -X github.com/lachlanharrisdev/gonetsim/cmd.Date=${BUILD_DATE}" \
      -o /out/gonetsim \
      .


FROM debian:bookworm-slim AS certgen

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/gonetsim
COPY --from=builder /src/internal/config/default_config.toml /etc/gonetsim/gonetsim.toml
COPY --from=builder /out/gonetsim /gonetsim
RUN /gonetsim tls --config /etc/gonetsim/gonetsim.toml

RUN cp /etc/gonetsim/gonetsim-ca.pem /usr/local/share/ca-certificates/gonetsim-ca.crt && update-ca-certificates


FROM gcr.io/distroless/static-debian12:nonroot

# prefers /etc/gonetsim/gonetsim.toml if present
# includes a default config and a persisted TLS cert/key/CA so the container is runnable with no mounts
COPY --from=certgen /etc/gonetsim/ /etc/gonetsim/
COPY --from=certgen /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/gonetsim /gonetsim

WORKDIR /work

EXPOSE 5353/udp
EXPOSE 5353/tcp
EXPOSE 8080/tcp
EXPOSE 8443/tcp
EXPOSE 1025/tcp
EXPOSE 1465/tcp

ENTRYPOINT ["/gonetsim"]
