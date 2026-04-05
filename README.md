![GitHub Repo stars](https://img.shields.io/github/stars/lachlanharrisdev/gonetsim?style=social)
![GitHub](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lachlanharrisdev/gonetsim)
![GitHub all releases](https://img.shields.io/github/downloads/lachlanharrisdev/gonetsim/total)
![GitHub CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)
![GitHub Release Status](https://img.shields.io/github/v/release/lachlanharrisdev/gonetsim)

# gonetsim

A WIP alternative to `inetsim` in go, designed as a lightweight alternative with much stronger support for modern operating systems

## Quick start

Run the main services:

```bash
go run . serve
```

Run just one service:

```bash
go run . dns
go run . http
go run . https
```

## Config

### serve

```bash
go run . serve \
	--dns-listen :5353 --dns-network udp --dns-ipv4 127.0.0.1 --dns-ipv6 ::1 \
	--http-listen :8080 --http-status 200 \
	--https-listen :8443 --https-status 200
```

Disable a service:

```bash
go run . serve --https=false
```

### HTTPS notes

- If `--cert/--key` are not provided, the program generates an ephemeral self-signed certificate

## Example checks

```bash
dig @127.0.0.1 -p 5353 example.com A
curl -v http://127.0.0.1:8080/
curl -vk https://127.0.0.1:8443/
```

`Copyright (c) 2026 Lachlan Harris. All Rights Reserved.`