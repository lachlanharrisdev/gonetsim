<div align="center">

  <h1 align="center">GoNetSim</h1>

  <p align="center" width="100">
    Go Network Simulator. A spiritual, unofficial successor to the <a href="https://www.inetsim.org/"><code>inetsim</code> project</a>, providing a suite of tools & services for simulating common internet services in a controlled environment.
    <br />
  </p>
  <div align="center" width="50">
    
  [![GitHub Repo stars](https://img.shields.io/github/stars/lachlanharrisdev/gonetsim?style=social)](https://github.com/lachlanharrisdev/gonetsim/stargazers)
  [![GitHub](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/?tab=MIT-1-ov-file)
  [![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/)
  [![GitHub CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)](https://github.com/lachlanharrisdev/gonetsim/actions)
  [![GitHub Release Status](https://img.shields.io/github/v/release/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/releases/latest)
  [![Go Report Card](https://goreportcard.com/badge/github.com/lachlanharrisdev/gonetsim)](https://goreportcard.com/report/github.com/lachlanharrisdev/gonetsim)
    
  </div>

</div>

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

## Configuration

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

## Contributing

Praetor follows most standard conventions for contributing, and accepts any contributions from documentation improvements, bug triage / fixes, small features or any updates for [issues in the backlog](https://github.com/lachlanharrisdev/gonetsim/issues?q=is%3Aissue). For more information on contributing please see [CONTRIBUTING.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md).

### Codespaces

Praetor has full support for Github Codespaces. These are recommended for small changes or devices with no access to a development environment. You can use the buttons below to open the repository in a web-based editor and get started.

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/lachlanharrisdev/gonetsim?quickstart=1)

### Dev Containers

We also have full support for Dev Containers. These provide a reproducible development environment that automatically isolates the project and installs the officially supported toolchain. 

Clicking the below button will open up VS Code on your local machine, clone this repository and open it automatically inside a development container.

[![Open in Dev Containers](https://img.shields.io/badge/Open%20In%20Dev%20Container-0078D4?style=for-the-badge&logo=visual%20studio%20code&logoColor=white)](https://vscode.dev/redirect?url=vscode://ms-vscode-remote.remote-containers/cloneInVolume?url=https://github.com/lachlanharrisdev/gonetsim)

### Local Development

For local development, please refer to [CONTRIBUTING.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md). Again, we follow most conventions so local development involves the standard flow of `fork-PR-merge`.

<br/>

---

<br/>

> This project is licensed under the MIT License. Please see [LICENSE](https://github.com/lachlanharrisdev/gonetsim?tab=MIT-1-ov-file) for more info.
>
> Copyright (c) 2026 Lachlan Harris. All Rights Reserved.
