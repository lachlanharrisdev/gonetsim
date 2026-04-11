ARG GO_VERSION=1.26.1

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
COPY third_party ./third_party
RUN go mod download

COPY . ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64

ARG VERSION=dev
ARG REVISION=dev
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-s -w -X github.com/lachlanharrisdev/gonetsim/cmd.Version=${VERSION} -X github.com/lachlanharrisdev/gonetsim/cmd.Revision=${REVISION} -X github.com/lachlanharrisdev/gonetsim/cmd.Date=${BUILD_DATE}" \
      -o /out/gonetsim \
      .


FROM gcr.io/distroless/static-debian12:nonroot

# prefers /etc/gonetsim/gonetsim.toml if present
# including default config keeps the container runnable with no mounts
COPY --from=builder /src/internal/config/default_config.toml /etc/gonetsim/gonetsim.toml

COPY --from=builder /out/gonetsim /gonetsim

WORKDIR /work

EXPOSE 5353/udp
EXPOSE 5353/tcp
EXPOSE 8080/tcp
EXPOSE 8443/tcp

ENTRYPOINT ["/gonetsim"]
