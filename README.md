# lxp-scan-svc

A Go **gRPC** service that fronts [`lxp-scan`](../lxp-scan) — the Rust cross-repo
FE analyzer — so tools (CI plugins, IDEs, agents) can pull ground truth over the
wire with **typed, structured responses**. The Rust analyzer is linked
**in-process via cgo** (`liblxp_scan`), not run as a subprocess.

```
gRPC client (CI plugin / grpcurl / probe)
      │  Impact / Context / Drift / Dupes / Clones  → typed messages
      ▼
lxp-scan-svc (Go)   [auth interceptor + optional TLS]
      │  cgo → lxp_scan_call(root, {"name","arguments","format":"json"})
      ▼
liblxp_scan.dylib  (Rust cdylib; ffi.rs reuses lxp-scan's tool dispatch + JSON renderers)
```

## Layout

| Path | What |
|---|---|
| `proto/lxpscan.proto` | Service + typed messages |
| `gen/lxpscanpb/` | Generated gRPC Go (`make proto`) |
| `internal/scan/` | cgo bridge (`scan.go`), typed decode (`tools.go`), C header |
| `lib/` | `liblxp_scan.dylib` (copied by `make lib`) |
| `server.go` / `main.go` / `auth.go` | gRPC server, JSON→proto mapping, bearer auth |
| `cmd/probe/` | Typed smoke-test client |
| `cmd/dump/` | Prints raw lxp-scan JSON per tool (debug) |

## Build & run

Two-step: the Rust lib first, then the Go binary.

```sh
make all          # = make lib proto build
make run          # or ./lxp-scan-svc
```

`make lib` runs `cargo build --release` in `../lxp-scan` and copies the cdylib
into `lib/`; the Go binary bakes an rpath to `lib/`. Rebuild the lib whenever
lxp-scan changes. Needs `CGO_ENABLED=1` and a C toolchain.

### Flags / config

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `-addr` | | `localhost:50051` | listen address |
| `-root` | `LXP_SCAN_ROOT` | `~/Leapxpert/FE` | default workspace (per-request `root` overrides) |
| `-token` | `LXP_SCAN_TOKEN` | `""` | require `authorization: Bearer <token>`; empty = no auth |
| `-tls-cert` / `-tls-key` | | `""` | enable TLS (both required together) |

## RPCs

All return typed messages (the exact shapes lxp-scan computes):

| RPC | Args | Returns |
|---|---|---|
| `Impact` | `symbol`, `from?` | `sites[]` — repo/file/line, source, refs, jsx_uses, jsx_props, jsx_lines |
| `Context` | `symbol`, `from?`, `sites?` | symbol totals, `prop_counts[]`, `definition`, `excerpts[]`, `same_name[]` |
| `Drift` | — | `rows[]` — pkg, `versions{repo→{version,source}}`, level |
| `Dupes` | — | `groups[]` — name, `sites[]`, repo_count |
| `Clones` | `symbol?`, `min_tokens?`, `same_file?` | `clusters[]` (members, token_count, sig, literals, notes), `npm_only_packages[]` |

Missing required args → `InvalidArgument`; a scan/library failure → `Internal`.

## Try it

```sh
# local, no auth
./lxp-scan-svc -root ~/Leapxpert/tools/lxp-scan/tests/fixtures/workspace &
go run ./cmd/probe

# with auth + TLS
./lxp-scan-svc -token secret123 -tls-cert cert.pem -tls-key key.pem &
go run ./cmd/probe -tls -ca cert.pem -token secret123
```

Reflection is on, so `grpcurl` works (pass `-H 'authorization: Bearer <token>'`
and `-cacert` when those are enabled):

```sh
grpcurl -plaintext -d '{"symbol":"Avatar"}' localhost:50051 lxpscan.v1.ScanService/Impact
```

## Notes / next

- Rust FFI lives in `../lxp-scan/src/ffi.rs` (branch `ffi`).
- Auth is a shared bearer token (fine for internal CI). For per-user identity,
  swap the interceptor for mTLS or JWT verification.
- Deploy later: static-link the Rust lib, or ship the dylib beside the binary
  and set the rpath / `DYLD_LIBRARY_PATH`.
