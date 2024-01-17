<p align="center">
  <img src="https://raw.githubusercontent.com/cert-manager/cert-manager/d53c0b9270f8cd90d908460d69502694e1838f5f/logo/logo-small.png" height="256" width="256" alt="cert-manager project logo" />
</p>

# cmctl - The cert-manager Command Line Tool

`cmctl` is a command line tool that can help you manage cert-manager and its resources inside your cluster.

## Documentation

The documentation for `cmctl` can be found on the [cert-manager website](https://cert-manager.io/docs/usage/cmctl/).

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
