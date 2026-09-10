<div align="center">

  <h1 align="center">GoNetSim</h1>

  <p align="center" width="100">
    A lightweight network simulator for malware analysis. Point a sandboxed Lua
    handler at a TCP or UDP port. Every connection is logged to the terminal and
    written to a pcapng file that opens in Wireshark.
  </p>

  [![GitHub License](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim?tab=Apache-2.0-1-ov-file)
  [![CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)](https://github.com/lachlanharrisdev/gonetsim/actions)

</div>

## What it is

GoNetSim simulates the network protocols a malware sample expects to see when it
phones home. There is no C2 server to install and no protocol libraries to pull
in. You write a small Lua file that implements the server side, or use one of the
built-in handlers.

```sh
gonetsim echo@:7777                 # built-in echo
gonetsim handlers/irc.lua@:6667     # Lua handler over TCP
gonetsim sink@:9999/udp             # built-in sink over UDP
```

Everything runs in a sandbox. Scripts cannot read files, run code they did not
define, or touch the host. Handlers keep state through `conn`, `handler` and
`global` key/value stores, and they annotate packets for Wireshark with
`capture:comment()`.

## Install

Grab the archive for your platform from [Releases](https://github.com/lachlanharrisdev/gonetsim/releases),
which covers linux, windows and darwin. A container image is published at
`ghcr.io/lachlanharrisdev/gonetsim` for linux/amd64 and arm64, and the repository
includes a `docker-compose.yml`.

That is the whole install. The binary embeds its documentation, so it needs no
network access, including in fully air-gapped labs.

## Usage

Every positional argument is a listener in the form `handler@addr[/tcp|/udp]`.

```sh
gonetsim handlers/smtp.lua@:2525 --pcap case.pcapng
gonetsim echo@:8443 --tls --tls-keylog keys.log
gonetsim handlers/irc.lua@:6667 sink@:9999/udp --log-level debug
```

The remaining flags are `--listen`, `--timeout`, `--tls`, `--tls-cert`,
`--tls-key`, `--tls-keylog`, `--pcap`, `--no-pcap`, `--log-level` and
`--log-format`. A capture is written to `./<id>.pcapng` in the current directory
unless you pass `--no-pcap`. TLS listeners are captured as ciphertext, so a
decryptable capture needs `--tls-keylog`. Run `gonetsim docs flags` for the full
list.

## Documentation

Documentation ships inside the binary and works offline.

```sh
gonetsim docs            # list topics
gonetsim docs capture    # the pcapng file and decrypting TLS
gonetsim docs lua-api    # the sandboxed handler API
gonetsim docs design     # how it is put together
```

The same `docs/*.md` files feed the live website, so one edit lands in both
places. Example handlers live in `handlers/`.

## Development

GoNetSim is written in Go and needs Go 1.26 or later to build. `go build ./...`
and `go vet ./...` must pass. There are no tests by policy for a project this
size. CI builds, vets, lints for real bugs only, and builds the container image.
Releases are cut from `v*` tags by GoReleaser.

## License

Apache 2.0. See [LICENSE](LICENSE).
