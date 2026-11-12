# Verifying downloads

Every release publishes `checksums.txt`, signed with [Sigstore cosign](https://docs.sigstore.dev/)
using GitHub's OIDC identity (no long-lived keys). It also publishes SPDX SBOMs and
[SLSA build provenance](https://slsa.dev/) for the CLI archives and packages, the
desktop app installers and the VS Code extension.

## The CLI

1. Check that the checksum file was produced by this repository's release workflow:

    ```sh
    cosign verify-blob \
      --bundle checksums.txt.sigstore.json \
      --certificate-identity 'https://github.com/roystondwork-dev/diskdiet/.github/workflows/release.yml@refs/heads/main' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
      checksums.txt
    ```

2. Check your download against it:

    ```sh
    sha256sum --ignore-missing -c checksums.txt     # Linux
    shasum -a 256 --ignore-missing -c checksums.txt # macOS
    ```

3. Optionally, check the build provenance with the GitHub CLI:

    ```sh
    gh attestation verify diskdiet_*_linux_amd64.tar.gz --repo roystondwork-dev/diskdiet
    ```

## The desktop app and the VS Code extension

```sh
shasum -a 256 --ignore-missing -c diskdiet-app_checksums.txt
gh attestation verify diskdiet-app_*_macos_universal.dmg --repo roystondwork-dev/diskdiet
gh attestation verify diskdiet-vscode-*.vsix --repo roystondwork-dev/diskdiet
```

## npm and PyPI

The npm package downloads the release archive for your OS on first use and checks it
against `checksums.json`, which the release workflow generates from the signed
`checksums.txt` after verifying its signature. The PyPI wheels are built from the same
verified archives. Both packages are published with trusted publishing (GitHub OIDC),
so no long-lived tokens exist; npm and PyPI show where each version was built.
