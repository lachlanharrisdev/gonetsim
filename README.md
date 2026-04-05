![GitHub Repo stars](https://img.shields.io/github/stars/lachlanharrisdev/gonetsim?style=social)
![GitHub](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lachlanharrisdev/gonetsim)
![GitHub all releases](https://img.shields.io/github/downloads/lachlanharrisdev/gonetsim/total)
![GitHub CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)
![GitHub Release Status](https://img.shields.io/github/v/release/lachlanharrisdev/gonetsim)

# gonetsim

A WIP "fork" of `inetsim` in Go, designed as a lightweight alternative with much stronger support for modern operating systems for beginners.

<br/>

## Quick start

`gontesim serve` will run all services

```bash
gonetsim serve
```

Alternatively, you can specify a single service to run

```bash
gonetsim dns
gonetsim http
gonetsim https
```

Or choose individual services to disable

```bash
gonetsim serve --https=false
```

<br/>

## Config

Configuration files are a WIP. For now, there are plenty of flags that can be specified when running commands. To see the full list, please run `gonetsim help <command>`. Here is the list of flags for the `gonetsim serve` command

```go
Usage:
  gonetsim serve [flags]

Flags:
      --dns                   enable DNS (default true)
      --dns-ipv4 string       DNS sinkhole IPv4 (default "127.0.0.1")
      --dns-ipv6 string       DNS sinkhole IPv6 (empty disables) (default "::1")
      --dns-listen string     DNS listen address (default ":5353")
      --dns-network string    DNS network: udp or tcp (default "udp")
  -h, --help                  help for serve
      --http                  enable HTTP (default true)
      --http-listen string    HTTP listen address (default ":8080")
      --http-status int       HTTP status code (default 200)
      --https                 enable HTTPS (default true)
      --https-cert string     HTTPS TLS cert PEM path (optional)
      --https-key string      HTTPS TLS key PEM path (optional)
      --https-listen string   HTTPS listen address (default ":8443")
      --https-status int      HTTPS status code (default 200)
```

<br/>

---

<br/>

> Copyright (c) 2026 Lachlan Harris. All Rights Reserved.