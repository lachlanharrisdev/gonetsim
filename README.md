<div align="center">

  <img src=".github/logo_transparent.png" width="100" height="135"/>

  <h1 align="center">GoNetSim</h1>

  <p align="center" width="100">
    Go Network Simulator. A spiritual, unofficial successor to the <a href="https://www.inetsim.org/"><code>inetsim</code> project</a>, providing a suite of tools for simulating common internet services in a controlled environment.
    <br />
  </p>
  <p align="center" width="50">
    
  [![GitHub Repo stars](https://img.shields.io/github/stars/lachlanharrisdev/gonetsim?style=social)](https://github.com/lachlanharrisdev/gonetsim/stargazers)
  [![GitHub](https://img.shields.io/github/license/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/?tab=Apache-2.0-1-ov-file)
  [![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/)
  [![GitHub CI Status](https://img.shields.io/github/actions/workflow/status/lachlanharrisdev/gonetsim/ci.yaml?branch=main&label=CI)](https://github.com/lachlanharrisdev/gonetsim/actions)<br/>
  [![GitHub Release Status](https://img.shields.io/github/v/release/lachlanharrisdev/gonetsim)](https://github.com/lachlanharrisdev/gonetsim/releases/latest)
  [![Go Report Card](https://goreportcard.com/badge/github.com/lachlanharrisdev/gonetsim)](https://goreportcard.com/report/github.com/lachlanharrisdev/gonetsim)
    
  </p>
</div>

<br/>

## Quick start

`gonetsim` runs the main services (DNS + HTTP + HTTPS).

```yaml
gonetsim
```

Alternatively, you can specify a single service to run

```yaml
gonetsim dns
gonetsim http
gonetsim https
```

<br/>

## Configuration

GoNetSim uses a TOML configuration file for most configuration, rather than forcing the memorisation of many flags.

On first run, if no config file is found, GoNetSim generates a default commented config file in `$XDG_CONFIG_HOME/gonetsim/config.toml` and uses it.

Default search locations:

- `/etc/gonetsim/gonetsim.toml`
- `$XDG_CONFIG_HOME/gonetsim/config.toml` (usually `~/.config/gonetsim/config.toml`)
- `./gonetsim.toml`

To use a specific config file:

```yaml
gonetsim --config /path/to/gonetsim.toml
```

<br/>

## Contributing

GoNetSim follows most standard conventions for contributing, and accepts any contributions from documentation improvements, bug triage / fixes, small features or any updates for [issues in the backlog](https://github.com/lachlanharrisdev/gonetsim/issues?q=is%3Aissue). For more information on contributing please see [CONTRIBUTING.md](https://github.com/lachlanharrisdev/gonetsim/blob/main/.github/CONTRIBUTING.md).

### Codespaces

GoNetSim has full support for Github Codespaces. These are recommended for small changes or devices with no access to a development environment. You can use the buttons below to open the repository in a web-based editor and get started.

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

> This project is licensed under the Apache 2.0 License. Please see [LICENSE](https://github.com/lachlanharrisdev/gonetsim?tab=Apache-2.0-1-ov-file) for more info.
>
> Copyright (c) 2026 Lachlan Harris. All Rights Reserved.
