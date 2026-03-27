# Installation

## go install

```bash
go install github.com/fruitriin/ccchain/cmd/ccchain@latest
```

## Build from source

```bash
git clone https://github.com/fruitriin/EnumaElish.git
cd EnumaElish
make build
# Binary is at ./ccchain
```

## GitHub Releases

Pre-built binaries for macOS and Linux are available on the [Releases page](https://github.com/fruitriin/EnumaElish/releases).

| Platform | Architecture | Binary |
|---|---|---|
| macOS | Apple Silicon (arm64) | `ccchain-darwin-arm64` |
| macOS | Intel (amd64) | `ccchain-darwin-amd64` |
| Linux | x86_64 (amd64) | `ccchain-linux-amd64` |
| Linux | ARM64 | `ccchain-linux-arm64` |

## Verify Installation

```bash
ccchain --version
```
