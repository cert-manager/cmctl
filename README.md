<p align="center">
  <img src="https://raw.githubusercontent.com/cert-manager/cert-manager/d53c0b9270f8cd90d908460d69502694e1838f5f/logo/logo-small.png" height="256" width="256" alt="cert-manager project logo" />
</p>

# cmctl - The cert-manager Command Line Tool

`cmctl` is a command line tool that can help you manage cert-manager and its resources inside your cluster.

## Documentation

The documentation for `cmctl` can be found on the [cert-manager website](https://cert-manager.io/docs/usage/cmctl/).

## Installation

> [!Note]
> These instructions are a copy of the [official installation instructions](https://cert-manager.io/docs/usage/cmctl/#installation).

### Homebrew

On Mac or Linux if you have [Homebrew](https://brew.sh/) installed, you can install `cmctl` with:

```sh
brew install cmctl
```

This will also install shell completion.

### Go install

If you have Go installed, you can install `cmctl` with:

```sh
go install github.com/cert-manager/cmctl/v2@latest
```

### Manual Installation

You need the `cmctl` file for the platform you're using, these can be found on our [cmctl GitHub releases page](https://github.com/cert-manager/cmctl/releases).
In order to use `cmctl` you need its binary to be accessible under the name `cmctl` in your `$PATH`. Run the following commands to set up the CLI. Replace OS and ARCH with your systems equivalents:

```sh
OS=$(uname -s | tr A-Z a-z); ARCH=$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/'); curl -fsSL -o cmctl https://github.com/cert-manager/cmctl/releases/latest/download/cmctl_${OS}_${ARCH}
chmod +x cmctl
sudo mv cmctl /usr/local/bin
# or `sudo mv cmctl /usr/local/bin/kubectl-cert_manager` to use `kubectl cert-manager` instead.
```

### Shell Completion

`cmctl` supports shell completion for most popular shells. To get help on how to enable shell completion, run the following commands:

```sh
$ cmctl completion --help
# or `kubectl cert-manager completion --help`
...
Available Commands:
  bash        Generate cert-manager CLI scripts for a Bash shell
  fish        Generate cert-manager CLI scripts for a Fish shell
  powershell  Generate cert-manager CLI scripts for a PowerShell shell
  zsh         Generation cert-manager CLI scripts for a ZSH shell

$ cmctl completion bash --help
To load completions:
Bash:
  $ source <(cmctl completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ cmctl completion bash > /etc/bash_completion.d/cmctl

  # macOS:
  $ cmctl completion bash > /usr/local/etc/bash_completion.d/cmctl
...
```

## Versioning

Before v2, `cmctl` was located in the cert-manager repository and versioned together with cert-manager.
Starting from v2, `cmctl` is versioned seperately from cert-manager itself.

## Release Process

Create a Git tag with a tagname that has a `v` prefix and push it to GitHub.
This will trigger the [release workflow].

1. Create and push a Git tag

```sh
export VERSION=v2.0.0-alpha.0
git tag --annotate --message="Release ${VERSION}" "${VERSION}"
git push origin "${VERSION}"
```

2. Wait for the [release workflow] to succeed.

3. Visit the [releases page], edit the draft release, click "Generate release notes", and publish the release.

[release workflow]: https://github.com/cert-manager/cmctl/actions/workflows/release.yaml
[releases page]: https://github.com/cert-manager/cmctl/releases
