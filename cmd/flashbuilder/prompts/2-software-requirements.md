# Specification of `flashbuilder` – Generator of `flash`, an ultra‑Fast Go 1.26+ Static‑Asset HTTP Server

**Version 18.0.0 (2026‑10‑01)**

---

## 1. Primary Goal

Generate a **single‑binary Go 1.26 program** that serves a set of static assets with the **fastest possible request‑processing hot‑path** while obeying the **KISS** (Keep‑It‑Simple‑Stupid) principle. The generated server must implement the fastest possible web server with **at most** one array bounds check, one `switch`, and one conditional‑GET `if` per request. Any computation that can be performed at generation time **must** become a compile‑time constant. The generator itself must be as simple as possible – using only the Go standard library plus three explicitly listed third‑party modules. The generator compresses assets using Brotli, AVIF, and WebP (via CGO libraries), keeps the most size‑effective variant, and generates a single `embed.go` file containing all embed‑eligible assets. Large assets are organized in a `www/` tree mirroring the source directory structure. The generator **never modifies** the source tree and **only creates** links in the output tree.

---

## 2. Scope & Assumptions

| Item | Detail |
|------|--------|
| **Target Go version** | Go 1.26 (released 2026). All generated code must compile with `go build` ≥ 1.26. |
| **Supported platform** | Linux AMD64 only – both generator and generated server are built and tested on this platform. |
| **CGO policy** | **Generator** requires CGO (Brotli, AVIF, WebP bindings). **Generated server** is pure Go – no CGO. |
| **Dependency policy (generator)** | Go standard library, `github.com/alecthomas/kong`, `github.com/kalafut/imohash`, `github.com/google/brotli/go/cbrotli`, `github.com/vegidio/avif-go`, `github.com/kolesa-team/go-webp/encoder`. |
| **Dependency policy (generated server)** | Go standard library, `github.com/alecthomas/kong`, `golang.org/x/net/http2`, `github.com/quic-go/quic-go/http3`. |
| **Read‑only source tree** | The **input** (first positional argument) is **never modified**. All link creation happens inside **output**. |
| **Cache filesystem requirement** | Cache directory must be on the **same filesystem** as output for hard‑links. |
| **Determinism** | Fixed alphabetical ordering of assets, deterministic module name (`flash`). TLS certificate randomness is the only source of nondeterminism. |
| **Log‑level flag** | `-v` (counter) – 0 = WARN (default), 1 = INFO, ≥2 = DEBUG. |
| **Environment overrides** | Every CLI flag may be overridden by `FLASHBUILDER_<UPPERCASE_FLAG>` (generator) or `FLASH_<UPPERCASE_FLAG>` (generated server). |
| **//go:embed behavior** | In Go 1.26, `//go:embed` follows symlinks by default and works correctly with hard links. The embedded content is the actual file content, not the link itself. |
| **Output structure** | Embed‑eligible assets consolidated in `assets/` sub‑directory with single `embed.go`. Large assets in `www/` tree mirroring input. |

### 2.1 CGO Requirements

The generator requires the following system libraries and build environment for CGO compilation:

| Requirement | Details |
|-------------|---------|
| **System packages** | `libbrotli-dev`, `libavif-dev`, `libwebp-dev` must be installed on the build system. |
| **Build environment** | `CGO_ENABLED=1` must be set during generator compilation. |
| **Linker flags** | The generator links against `-lbrotlienc`, `-lavif`, `-lwebp` when building CGO bindings. |
| **Platform** | CGO bindings are platform‑specific; the generator must be built on Linux AMD64 with a working C toolchain. |

---

## 3. Glossary

| Term | Definition |
|------|------------|
| **input** | First mandatory positional argument – directory containing the **asset‑tree**. Never modified. |
| **output** | Second mandatory positional argument – destination for generated files, links, and Go source code. |
| **Asset** | Any regular file (or symlink to regular file) under input. |
| **Variant** | Transformed representation: Brotli‑compressed (`.br`), AVIF‑encoded (`.avif`), or WebP‑encoded (`.webp`). |
| **Embed‑eligible** | Asset (original or variant) selected by the embed‑budget algorithm, compiled into binary via `//go:embed`. |
| **Large asset** | Asset **not** embed‑eligible; served via per‑request `http.ServeFile` from `www/` tree. |
| **Canonical path** | POSIX‑style relative path discovered from input (e.g., `css/main.css`). |
| **Shortcut** | Canonical path without extension (non‑index) or without `/index.html` (index files). Root index shortcut is `""`. |
| **Router** | Compile‑time generated length‑based dispatch array plus per‑length `switch` mapping raw request path to handler. |
| **Cache directory** | Follows XDG Base Directory Specification (`$XDG_CACHE_HOME/flashbuilder` or `$HOME/.cache/flashbuilder` or `./.cache/flashbuilder`). Holds transformed variants. Must be on same filesystem as output. |
| **MaxLen** | Compile‑time constant: byte length of longest canonical, shortcut, or duplicate path (no leading `/`). |
| **assets/** | Sub‑directory in output containing all embed‑eligible assets as links, with single `embed.go`. |
| **www/** | Sub‑directory in output containing large assets in tree structure mirroring input. |
| **Hot path** | Request processing path containing at most one array bounds check, one `switch`, one conditional‑GET `if`. |

---

## 4. CLI Interface

Both the generator and generated server use **`github.com/alecthomas/kong`** for flag parsing. All flags and environment variables are processed exclusively through `kong` – no manual `os.Getenv` calls are permitted.

### 4.1 Generator CLI (`flashbuilder`)

**Usage:**
```
flashbuilder <input> <output> [flags]
```

**Positional Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| **input** | string | Path to asset‑tree (read‑only, never modified). |
| **output** | string | Destination for generated files and links. Must **not** equal input. Created if missing. |

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--embed-budget` | size | `0` | Cumulative size limit for embed‑eligible assets. **`0` means unlimited**. |
| `--brotli` | int | `11` | Brotli compression level (0‑11). Negative → disabled. |
| `--avif` | int | `50` | AVIF quality (0‑100). Negative → disabled. |
| `--webp` | int | `50` | WebP quality (0‑100). Negative → disabled. |
| `--csp` | string | `"default-src 'self'"` | Fallback CSP header. When `--csp=""` is explicitly set, CSP header is disabled. When set to a non-empty string, that string is used as the CSP header fallback value for HTML assets only. When unset, the default secure default CSP fallback is used. |
| `-v` | counter | `0` | Log level (0 = WARN, 1 = INFO, ≥2 = DEBUG). |
| `--dry-run` | bool | `false` | Simulate full pipeline **without** writing to output. |
| `--tests` | bool | `false` | Run `go test ./... -race -vet=all` after generation. |
| `--cache-max` | size | `5 GiB` | Cache directory upper bound; excess evicted (oldest‑modified first). |
| `--cache-dir` | path | `CACHE_DIR` | Cache directory path. |
| `--version` | bool | `false` | Print version and exit. |
| `--help` | bool | `false` | Show usage. |

`CACHE_DIR` follows XDG Base Directory Specification: `$XDG_CACHE_HOME/flashbuilder` if `$XDG_CACHE_HOME` is set; otherwise `$HOME/.cache/flashbuilder` if `$HOME` is set; otherwise `./.cache` (current directory). The Cache directory must be on the same filesystem as output for hard-links.
### 4.2 Generated‑Server CLI (`flash`)

**Usage:**
```
flash [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--http` | string | `localhost:8080` | Plain‑HTTP bind address (`none` to disable). Supports HTTP/1.1 + h2c Upgrade. |
| `--https` | string | `localhost:8443` | TLS 1.3 bind address (`none` to disable). Enables HTTP/2 via ALPN and HTTP/3 via QUIC. |
| `--admin` | string | `localhost:8081` | Admin listener address (`none` disables). |
| `-v` | counter | `0` | Log level (0 = WARN, 1 = INFO, ≥2 = DEBUG). |
| `--help` | bool | `false` | Show usage. |

---

## 5. Functional Requirements

All requirements are individually labeled for machine parseability.

### FR‑1: Asset Discovery

Recursively walk the **input** directory. Follow symlinks to regular files; never follow symlinks to directories. Ignore sockets, device files, and FIFOs without warning. Record size, modification time, MIME type, and `imohash` for every discovered asset.

### FR‑2: MIME‑type Detection

MIME detection follows this exact order:
1. **Extension‑based lookup**: `mime.TypeByExtension` is called first using the file extension. Skip this step for files with no extension.
2. **Content sniffing**: If the extension‑based lookup returns empty, sniff the first 512 bytes with `http.DetectContentType`.
3. **Fallback**: If still unknown, fall back to `application/octet-stream`.

An asset is considered **HTML** when its detected MIME type is `text/html` or any variant (e.g., `text/html;charset=utf‑8`).

### FR‑3: Content‑hash Deduplication

Assets with identical `imohash` and byte content share a single canonical identifier. **Deduplication occurs BEFORE budget calculation**. Duplicates are resolved via router mapping only – no files are created in `assets/` or `www/`. The router maps duplicate request paths to the canonical handler.

### FR‑4: Embed‑budget Allocation

Greedy "smallest‑first" selection until cumulative size would exceed `--embed-budget`. **`--embed-budget=0` means unlimited**. Use `--embed-budget=1` to effectively disable embedding (embedding disabled when budget is insufficient for any asset). Deduplication is applied BEFORE budget calculation.

### FR‑5: Brotli Variant

Enabled when `--brotli ≥ 0`. Applies **only** to assets ≥ 2 KiB whose MIME is **NOT** `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/avif` (includes SVG and ICO). Keep `.br` variant **iff** `(variantSize < originalSize - 3*1024) OR (variantSize < 0.9 * originalSize)`. This means: keep variant if size reduction exceeds 3 KiB OR reduction exceeds 10% of original size. If kept: (a) reuse or create variant in Cache directory, (b) create **hard‑link** in `assets/` pointing to variant (absolute path), (c) **do not** create symbolic link for original. If condition fails: embed original, create symbolic link in `assets/` pointing to original (relative path).

### FR‑6: AVIF & WebP Variants

Enabled when `--avif ≥ 0` and/or `--webp ≥ 0`. Applies **only** to `image/png` and `image/jpeg`. Encode at specified quality. Keep **smallest** variant **iff** `(variantSize < originalSize - 3*1024) OR (variantSize < 0.9 * originalSize)`. This means: keep variant if size reduction exceeds 3 KiB OR reduction exceeds 10% of original size. If kept: (a) reuse or create variant in Cache directory, (b) create **hard‑link** in `assets/` pointing to variant (absolute path), (c) **do not** create symbolic link for original. If condition fails: embed original, create symbolic link in `assets/` pointing to original (relative path).

### FR‑7: Header Pre‑computation

Pre‑compute HTTP response headers **only** for embed‑eligible assets. Large assets served via `http.ServeFile` do **not** receive pre‑computed headers. `Cache-Control`: canonical index → `public, max‑age=31536000, immutable, must‑revalidate`; all others → `public, max‑age=31536000, immutable`. CSP added **only** for HTML assets. When `--csp=""` is explicitly set, CSP header is disabled (no CSP header emitted). When `--csp` is unset, a default CSP fallback is applied: `"default-src 'self'"`. HSTS added to every TLS response.

### FR‑8: Identifier Generation

Produce a unique exported Go identifier prefixed with `Asset` for each embed‑eligible canonical file. Sanitise relative path: split on `/` and `.`; capitalise first rune of each segment; drop non‑alphanumeric characters. Resolve collisions by appending `_` + zero‑padded integer (`_001`, `_002`, …). The identifier is also the **filename base** in `assets/`.

### FR‑9: TLS Certificate

At server start‑up, generate a self‑signed RSA 2048‑bit certificate with a random 64‑bit serial and a fixed 10‑year validity period.

### FR‑10: Router Generation with Frequency‑Ordered Switch Cases

Build a compile‑time `dispatch` array of size `MaxLen+1`. Each entry points to a per‑length handler. Router extracts raw request path via `r.URL.Path[1:]` (no leading `/`). **Exact‑length matching first**: if request path length matches a registered path length, invoke per‑length handler. If no exact match, search backwards for the largest route path that is a prefix of the request path. Dispatch array fully populated – no `nil` entries. Path matching uses raw byte comparison; URL-encoded characters are not decoded before matching. Both canonical and duplicate paths are included in the router's switch statements, both calling the canonical handler from the `assets` package.

**Switch case ordering optimization**: For each per‑length handler `getLenL`, the generator estimates the relative access frequency of each route and orders `case` statements by descending estimated frequency to minimize average comparison count. The ordering is performed independently for each length value `L`, considering only routes of that exact byte length. Routes with higher estimated frequency appear earlier in the switch statement, reducing the average number of string comparisons for frequently accessed assets.

### FR‑11: Admin Server

When `--admin` is not `none`, start a plain‑HTTP admin listener exposing `/healthz` (liveness probe, returns `200 OK` when server is healthy, `503 Service Unavailable` when unhealthy) and `/readyz` (readiness probe, returns `200 OK` when server is ready to accept traffic, `503 Service Unavailable` when not ready) and `/version` (liveness).

### FR‑12: Plain‑HTTP Listener

Bind to `--http` (unless `none`). Supports HTTP/1.1 and h2c Upgrade via `golang.org/x/net/http2`.

### FR‑13: HTTPS & QUIC Listener

Bind to `--https` (unless `none`). Enables HTTP/2 via ALPN and starts a QUIC listener on the same host:port for HTTP/3 via `github.com/quic-go/quic-go/http3`. HTTP/3 requires UDP listener on the same port as HTTPS TCP listener.

### FR‑14: Graceful Shutdown

On `SIGINT`/`SIGTERM`, stop accepting new connections and allow up to **5 seconds** for in‑flight requests. On `SIGHUP`, cycle log level (WARN → INFO → DEBUG → WARN).

### FR‑15: Logging

Generated server uses `log/slog` with the log level selected by `-v`.

### FR‑16: Testing

After a successful generation (or dry‑run), run `go test ./... -race -vet=all` inside output. Failure aborts with exit status 2 (`E079`).

### FR‑17: Dry‑run

Simulate the full pipeline without writing files under output. The cache directory may be accessed.

### FR‑18: Deterministic Builds

Fixed alphabetical ordering of assets, deterministic module name (`flash`). TLS certificate randomness is the only source of nondeterminism.

### FR‑19: No Runtime I/O for Embed‑eligible Assets

Embed‑eligible assets served from in‑memory `[]byte` literals via the `assets` package; no filesystem reads. Large assets served via per‑request `http.ServeFile` from `www/`.

### FR‑20: Zero‑allocation per Request

Header block written with a single `Write`; body written with a single `Write` (or `http.ServeFile` for large assets).

### FR‑21: Flag & Env Overrides

Every CLI flag may be overridden by `FLASHBUILDER_…` (generator) or `FLASH_…` (generated server) environment variables. All flag parsing is performed exclusively through `kong`.

### FR‑22: Blocked‑port Enforcement

Generated server rejects any bind address whose port appears in the blocked‑port list (Appendix A), exits with **E010**.

### FR‑23: Output‑tree Population

Link creation follows conditional logic. (See **Section 8.10: Link Creation Logic** for detailed cases).

### FR‑24: Cache Management

Cache stores raw bytes of each transformed variant. When a variant is needed, the generator first checks the cache; if present, it hard‑links the cached file into `assets/`. After generation, if total cache size exceeds `--cache-max`, evict oldest‑modified files until under the limit.

### FR‑25: Conditional GET Handling

For embed‑eligible assets, the generated handler performs **one** `if` that checks both `If‑None‑Match` and `If‑Modified‑Since` and returns `304 Not Modified` when appropriate. Large assets rely on `http.ServeFile`'s built‑in handling.

### FR‑26: Zero‑allocation Path Handling

Router extracts request path via `r.URL.Path[1:]` (raw slice) with **no heap allocation**.

### FR‑27: Admin Health‑check Semantics

`/healthz` returns `200 OK` when server is healthy, `503 Service Unavailable` when unhealthy. `/readyz` returns `200 OK` when server is ready to accept traffic, `503 Service Unavailable` when not ready. `/version` returns version information.

### FR‑28: FuzzIdentifierCollision

Fuzz test feeding random strings to identifier generator, ensuring no panic and valid Go identifiers.

### FR‑29: FuzzRouterPath

Fuzz test feeding random URL paths to router, ensuring no panic and correct 404 handling.

### FR‑30: Cache‑eviction Policy

Oldest‑modified eviction when cache exceeds `--cache-max`.

### FR‑31: Dispatch‑array Non‑nil Guarantee

All `dispatch` entries are non‑nil. If no canonical or shortcut path matches length 0, set `dispatch[0] = http.NotFound`. Otherwise, `dispatch[0]` serves the root `index.html` (or its kept variant).

### FR‑32: Router Out‑of‑range Fallback

When a request path is longer than any known route, the router searches backwards for the largest route path that is a prefix of the request path. **Prefix matching applies to all paths** (both embed‑eligible and large asset paths). The default case of each per‑length handler calls `dispatch[L‑1]`. If no shorter length exists, the chain ends at `dispatch[0]`.

### FR‑33: Cross‑compilation Disclaimer

Generator designed for Linux AMD64; cross‑compiling to other OS/arch is **not supported**.

### FR‑34: Version JSON Payload

`/version` returns a JSON object containing generator version, build time, asset counts, embed size, variant counts, and effective bind addresses.

### FR‑35: Protocol Support

All listeners advertise supported protocols via ALPN or Upgrade headers.

### FR‑36: Alt‑Svc Header

Every TLS response includes `Alt‑Svc: h3=":<httpsPort>"; ma=2592000`. Omitted if TLS is disabled.

### FR‑37: HSTS Unconditional

`Strict‑Transport‑Security` is emitted on **every** TLS response, never on plain‑HTTP.

### FR‑38: Signal Handling

`SIGHUP` cycles the log level (WARN → INFO → DEBUG → WARN). `SIGINT` and `SIGTERM` trigger graceful shutdown with a 5‑second timeout.

### FR‑39: CGO Mandatory (generator only)

Generator requires CGO enabled and a working C tool‑chain. Generated server is pure Go.

### FR‑40: Concurrent Generator Runs

Generator supports parallel executions; cache uses independent files and link creation is atomic, therefore no lock file is required.

### FR‑41: Deterministic Module Name

Generator runs `go mod init flash` and `go mod tidy`. The `go` toolchain must use the latest compatible versions of all dependencies – this ensures reproducibility through the `go.mod` file generated by `go mod tidy` while allowing automatic updates to the latest compatible versions. The purpose is to balance reproducibility with security: `go mod tidy` records the exact versions needed for compilation while allowing future builds to use newer compatible versions automatically.

### FR‑42: No Runtime Cache Usage

Generated server never reads from the cache directory; assets are either embedded (from `assets/`) or served from `www/`.

### FR‑43: No External Configuration File

All configuration is supplied via CLI flags or environment variables.

### FR‑44: No Runtime Compression/Transcoding

All variants are pre‑computed; the server never calls Brotli/AVIF/WebP at runtime.

### FR‑45: Method Validation

Only `GET` and `HEAD` are accepted; any other method returns `405 Method Not Allowed` without a body.

### FR‑46: Allowed Imports

Generated server may import **only**: Go standard library, `github.com/alecthomas/kong`, `golang.org/x/net/http2`, `github.com/quic-go/quic-go/http3` (the `http3` submodule). The `assets` package is a local package. Import statements must be validated during generation to ensure no unauthorized imports are present.

### FR‑47: Flag Parsing via Kong Only

All flag and environment variable parsing is performed exclusively through `kong`. No manual `os.Getenv` calls are permitted anywhere in the generated code.

### FR‑48: Single embed.go Generation

Generator creates a **single** `embed.go` file in the `assets/` package. (See **Section 7.4: embed.go Template Structure** for the exact template).

### FR‑49: Asset Compression Prioritisation

When multiple variants satisfy the keep condition, keep the **smallest** variant. Only one variant per asset.

### FR‑50: Source‑tree Read‑only Guarantee

Generator **never modifies, deletes, or moves** any file in input. All link creation happens inside output.

### FR‑51: Handler Function Generation

Each canonical path, shortcut path, and duplicate path has a dedicated handler function. Embed‑eligible handlers are in `assets/embed.go`; large asset handlers are in the main router file calling `http.ServeFile(w, r, "www/<relpath>/<filename>")`. Duplicate paths are mapped in the router to call the canonical handler from `assets`. For each entry in `duplicatePaths`, the router switch statement includes a case that calls `assets.ServeAssetCanonical(w, r)` where `Canonical` is the resolved canonical identifier from the map.

### FR‑52: Router Path Maps

Generator uses three `map[string]string` maps: `canonicalPaths` maps canonical path to identifier; `shortcutPaths` maps shortcut path to identifier; `duplicatePaths` maps duplicate path to canonical identifier. For each entry in `duplicatePaths`, the router generates a switch case that invokes the canonical handler. For example, `"images/logo.png":"AssetLogoPng"` generates `case "images/logo.png": assets.ServeAssetLogoPng(w, r)` in the appropriate per‑length handler.

### FR‑53: Shortcut Collision Detection

Before inserting any entry into `canonicalPaths`, `duplicatePaths`, or `shortcutPaths`, check all maps; skip if exists. Shortcuts colliding with actual canonical or duplicate files are **silently dropped**.

### FR‑54: Import Resolution

Main router file imports the `assets` package (local). All embed‑eligible handler calls use `assets.ServeAssetXXX(w, r)`. Large asset handlers are generated in the main router file.

### FR‑55: Large Asset Tree Structure

The `www/` sub‑directory mirrors the input relative path structure for large assets only. Intermediate directories are created as needed. Empty directories are pruned after link creation. The `www/` tree contains only symbolic links to original files in input.

### FR‑56: Identifier Filename Mapping

For embed‑eligible assets in `assets/`, the filename is constructed as `Asset<CamelCase>.<extension>` where `<extension>` is the original file extension for originals or the variant extension for variants. The identifier and filename base are identical, enabling `//go:embed` to reference the file directly.

### FR‑57: No Empty assets/ Directory

If there are no embed‑eligible assets, the `assets/` directory is not created and no `embed.go` file is generated.

### FR‑58: No Empty www/ Directory

If there are no large assets, the `www/` directory is not created.

### FR‑59: Directory Creation Order

Generator creates `assets/` first (if embed‑eligible assets exist), then `www/` (if large assets exist). Both are under output.

### FR‑60: Link Path Calculation

Symbolic links in `assets/` point to original files in input using relative paths calculated via `filepath.Rel(output/assets/, input/<relpath>/<filename>)`. Hard links in `assets/` point to variants in Cache directory using absolute paths. Symbolic links in `www/` point to original files in input using relative paths calculated via `filepath.Rel(output/www/<relpath>/, input/<relpath>/<filename>)`. The exact algorithm uses Go's `filepath.Rel` function with proper error handling for edge cases including deep directory structures and same-directory links.

### FR‑61: Variant Extension Mapping

For variants, the file extension in `assets/` is determined by the variant type: Brotli → `.br`, AVIF → `.avif`, WebP → `.webp`. The identifier base remains the same as the original path.

### FR‑62: //go:embed Follows Symlinks

In Go 1.26, `//go:embed` follows symlinks by default and works correctly with hard links. When `//go:embed` references a symlink in the `assets/` directory, it follows the symlink to embed the content from the original file in `input` or the variant in Cache directory. Hard links embed the content directly from the cache. The embedded content is the actual file content, not the link itself.

### FR‑63: Relative Path Calculation

Relative paths for symlinks are computed using Go's `filepath.Rel(from, to)` function. For symlinks in `assets/`, the relative path is calculated from `output/assets/` to `input/<relpath>/<filename>`. For symlinks in `www/`, the relative path is calculated from `output/www/<relpath>/` to `input/<relpath>/<filename>`. Hard links in `assets/` use absolute paths to Cache directory. Edge cases handled include: deep directory structures (paths with many components), same-directory links (when `from` and `to` share common prefixes), and cross-directory links. Error handling includes validation that computed paths are valid and point to existing files.

### FR‑64: CGO Bindings

Generator uses: `github.com/google/brotli/go/cbrotli` for Brotli compression, `github.com/vegidio/avif-go` for AVIF encoding, `github.com/kolesa-team/go-webp/encoder` for WebP encoding. The generator build requires `CGO_ENABLED=1` with system libraries `libbrotli`, `libavif`, and `libwebp` installed.

---

## 6. Non‑Functional Requirements

All non‑functional requirements are individually labeled for machine parseability.

### NFR‑1: Performance

Routing must be O(1): one array lookup → one `switch` → one conditional‑GET `if`. Benchmarks must verify zero heap allocations for the hot path. Benchmark tests must report allocations using `go test -bench=. -benchmem` with output showing 0 allocations per request for embed‑eligible assets.

### NFR‑2: Memory

No artificial limit on embed‑budget; the generated server must allocate zero heap per request for embed‑eligible assets. Large assets served via `http.ServeFile` may allocate per request.

### NFR‑3: Binary Size

Only embed‑eligible assets (original or kept variant) are compiled into the binary via `//go:embed` in `assets/embed.go`.

### NFR‑4: Portability

Generator runs on Linux AMD64 only.

### NFR‑5: Security

All responses must include the strict header set (CSP for HTML, HSTS on TLS, X‑Content‑Type‑Options, X‑Frame‑Options for HTML).

### NFR‑6: Observability

Text logs with timestamps, level, and structured fields; generated server uses JSON‑encoded logs.

### NFR‑7: Reliability

Server timeouts: `ReadHeaderTimeout:5s`, `ReadTimeout:10s`, `WriteTimeout:30s`, `IdleTimeout:120s`.

### NFR‑8: Determinism

Fixed alphabetical ordering of assets, deterministic module name.

### NFR‑9: Dependency Policy

Generator does **not** pin versions; `go mod tidy` fetches the latest compatible releases.

### NFR‑10: Testability

`--tests` runs a full suite (unit, integration, fuzz, benchmark).

### NFR‑11: Code Quality

All generated source files must pass `go vet` and `staticcheck`.

### NFR‑12: Graceful Shutdown

Must complete in‑flight requests within 5 seconds after SIGINT/SIGTERM.

### NFR‑13: CGO (generator only)

Generator requires CGO. Generated server is pure Go.

### NFR‑14: KISS Compliance

Both generator and generated server must remain easy to read, reason about, and maintain. When a trade‑off is required, **runtime performance always takes precedence** over brevity.

### NFR‑15: Single embed.go File

All embed‑eligible assets consolidated in one `embed.go` file, simplifying generated code structure and reducing import complexity.

### NFR‑16: Separated Asset Trees

Embed‑eligible assets isolated in `assets/`, large assets isolated in `www/`, providing clear separation.

---

## 7. Data Model

### 7.1 Asset Struct

```go
type Asset struct {
    RelPath        string        // POSIX‑style path relative to input, e.g. "css/main.css"
    AbsPath        string        // Absolute source path (used for symlink creation)
    Size           int64         // Raw file size in bytes
    ModTime        time.Time     // Modification time (UTC), used to pre-compute Last-Modified
    MIME           string        // Determined MIME type
    ImoHash        uint64        // imohash of raw content, used to pre-compute ETag
    IsDuplicate    bool          // True if another asset shares identical content
    CanonicalID    string        // Identifier of the canonical asset (if duplicate)
    EmbedEligible  bool          // True if selected by embed‑budget (original or kept variant)
    Variants       []Variant     // Kept variants (max one per type)
    HeaderLiteral  []byte        // Pre‑computed header block (embed‑eligible only)
    Identifier     string        // Exported Go identifier (prefixed with "Asset")
    Filename       string        // Filename in assets/ directory (e.g., "AssetMainCss.css")
    FrequencyScore int           // Estimated request traffic frequency
}
```

### 7.2 Variant Struct

```go
type Variant struct {
    Type          VariantType   // Brotli, AVIF, WebP
    Size          int64
    HeaderLiteral []byte        // Pre‑computed header block for variant
    Identifier    string        // Exported Go identifier for variant handler
    Extension     string        // File extension (e.g., "br", "avif", "webp")
    CachePath     string        // Cache directory path
}
```

### 7.3 VariantType Constants

```go
type VariantType int

const (
    VariantBrotli VariantType = iota
    VariantAVIF
    VariantWebP
)
```

### 7.4 embed.go Template Structure

The `embed.go` file is generated in the `assets/` package and contains all embed‑eligible assets. The file follows this exact template:

```go
package assets

import (
    _ "embed"
    "net/http"
)

//go:embed AssetAboutIndexHtml.html
var AssetAboutIndexHtml []byte

//go:embed AssetMainCssCss
var AssetMainCss []byte

//go:embed AssetLogoPngAvif
var AssetLogoPng []byte

// ... all embed‑eligible assets ...

var AssetAboutIndexHtmlHeader []byte = []byte("...")
var AssetMainCssHeader []byte = []byte("...")
// ... all header literals ...

func ServeAssetAboutIndexHtml(w http.ResponseWriter, r *http.Request) {
    _,_ = w.Write(AssetAboutIndexHtmlHeader)
    if len(r.Method) == 0 || r.Method[0] != http.MethodHead[0] {
        _,_ = w.Write(AssetAboutIndexHtml)
    }
}

func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    _,_ = w.Write(AssetMainCssHeader)
    if len(r.Method) == 0 || r.Method[0] != http.MethodHead[0] {
        _,_ = w.Write(AssetMainCss)
    }
}

// ... all handler functions ...
```

---

## 8. Generation Workflow

All steps are deterministic, use only allowed dependencies, and obey KISS.

### 8.1: Validate CLI Arguments

Ensure input exists and is a directory, output is different from input and not a sub‑directory of it. Create output if missing. Verify that Cache directory is on the same filesystem as output (abort with **E087** if not). All flag parsing is performed through `kong` with validation for all flag values.

### 8.2: Asset Discovery

Walk input (no directory symlink following). Record metadata for each regular file or symlink‑to‑file.

### 8.3: MIME Detection

Resolve MIME type for each asset using the exact order: extension‑based lookup via `mime.TypeByExtension`, content sniffing via `http.DetectContentType`, fallback to `application/octet-stream`. Handle files with no extension through content sniffing only.

### 8.4: Content‑hash Deduplication

Build `hashMap` → canonical assets. Mark duplicates, record canonical paths. Deduplication occurs BEFORE budget calculation. Duplicates are resolved via router mapping only – no files are created.

### 8.5: Variant Generation

**Brotli:** Assets ≥ 2 KiB, MIME NOT `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/avif` (includes SVG, ICO). **AVIF/WebP:** ONLY `image/png` and `image/jpeg`. Use worker pool. For each unique asset (by hash), generate enabled variants, store in Cache directory. Apply the keep condition; if kept, mark as selected representation.

### 8.6: Select Most‑compact Variant

If multiple variants satisfy keep condition, keep **smallest**. Only one variant per asset.

### 8.7: Embed‑budget Allocation

Sort selected assets by size (ascending). Deduplication already applied. Greedily accumulate until budget limit (or unlimited if `--embed-budget=0`). Mark `EmbedEligible = true`.

### 8.8: Header Pre‑computation

Build single `[]byte` header literal for each embed‑eligible asset/variant. Large assets do NOT receive pre‑computed headers; served via `http.ServeFile`.

### 8.9: Identifier Generation

Produce deterministic Go identifier for each embed‑eligible canonical file/variant, handling collisions. Generate filename for `assets/`: `Asset<CamelCase>.<extension>` for originals, `Asset<CamelCase>.<variant-ext>` for variants.

### 8.10: Link Creation Logic

The generator **never deletes** any file in input or output; it only creates new links. The conditional logic for link creation follows these cases:

**Case 1: Canonical embed‑eligible asset with kept variant**
- Create a **hard‑link** in `assets/` as `Asset<CamelCase>.<variant-ext>` (e.g., `AssetMainCss.br`) pointing to the variant file in Cache directory using an **absolute path**.
- Do **not** create a symbolic link for the original.

**Case 2: Canonical embed‑eligible asset without variant**
- Create a **symbolic link** in `assets/` as `Asset<CamelCase>.<ext>` (e.g., `AssetAboutIndexHtml.html`) pointing to the original file in input using a **relative path** calculated via `filepath.Rel(output/assets/, input/<relpath>/<filename>)`.

**Case 3: Large asset**
- Create a **symbolic link** in `www/<relpath>/<filename>` pointing to the original file in input using a **relative path** calculated via `filepath.Rel(output/www/<relpath>/, input/<relpath>/<filename>)`.
- Create intermediate directories in `www/` as needed to preserve the relative path structure.
- Prune empty directories in `www/` after link creation.

**Case 4: Duplicate asset**
- **No file creation**; the router maps the duplicate path to the canonical identifier's handler.

Edge cases handled in path calculation:
- Deep directory structures with many components
- Same-directory links where `from` and `to` share common prefixes
- Cross-directory links with no common components
- Validation that computed paths are valid and point to existing files

Error handling includes:
- Abort with **E087** if link creation fails
- Abort with **E087** if `filepath.Rel` calculation fails
- Abort with **E087** if hard-link target is on different filesystem

### 8.11: Build Path Maps

Build three `map[string]string` maps: `canonicalPaths`, `shortcutPaths`, and `duplicatePaths`. Check all maps before insertion; skip if exists. Shortcuts colliding with actual canonical or duplicate files are silently dropped. Each map entry generates corresponding router switch cases.

### 8.12: Generate embed.go

If embed‑eligible assets exist, generate single `embed.go` in `assets/` package containing all `//go:embed` directives, header literals, and handler functions. `//go:embed` follows symlinks and hard links to embed actual content. If no embed‑eligible assets, skip (no `assets/` directory created).

### 8.13: Router Generation with Frequency‑Ordered Switch Cases

Compute MaxLen (byte length of longest path without leading `/`). Generate dispatch array and per‑length handlers. For each per‑length handler `getLenL`, estimate the relative access frequency of each route and order `case` statements by descending frequency to minimize average comparison count. (See **Section 9: Router Implementation** for details).

### 8.14: TLS Configuration

Emit TLS‑certificate generation function (random serial, 10‑year validity).

### 8.15: Protocol Listeners

Generate listener setup, Alt‑Svc header injection, graceful‑shutdown logic. HTTP/3 listener uses UDP on the same port as HTTPS TCP listener.

### 8.16: Module Initialization

Run `go mod init flash` and `go mod tidy`. The `go mod tidy` command uses the latest compatible versions of all dependencies, recorded in `go.mod` for reproducibility.

### 8.17: Build

Invoke `go build -trimpath -ldflags="-s -w" -o flash ./...` inside output.

### 8.18: Cache Eviction

After generation, compute total cache size; if exceeds `--cache-max`, evict oldest‑modified files.

### 8.19: Dry‑run Handling

If `--dry-run`, perform steps 8.1‑8.18 **without** writing files under output. Cache may be accessed.

### 8.20: Testing

After successful generation (or dry‑run), run `go test ./... -race -vet=all` inside output. Abort with **E079** on failure.

### 8.21: Exit

Return `0` on success, appropriate non‑zero exit code on failure.

---

## 9. Router Implementation

### 9.1: MaxLen Computation

Generator computes `MaxLen` as the **byte length** of the longest canonical, shortcut, or duplicate path (no leading `/`). `MaxLen` is a compile‑time constant.

### 9.2: Dispatch Array Structure

```go
var dispatch [MaxLen+1]func(http.ResponseWriter, *http.Request)
```

### 9.3: Path Maps Construction

Generator builds three `map[string]string` maps:

1. **canonicalPaths** – Maps canonical path to identifier: `"css/main.css":"AssetMainCss"`.
2. **shortcutPaths** – Maps shortcut path to identifier: `"about":"AssetAboutIndexHtml"`.
3. **duplicatePaths** – Maps duplicate path to canonical identifier: `"images/logo.png":"AssetLogoPng"`.

**Collision avoidance:** Before inserting any entry, check all maps; skip if exists. Shortcut colliding with actual canonical or duplicate file is silently dropped.

**Duplicate handling:** For each entry in `duplicatePaths`, the router generates a switch case that invokes the canonical handler. For example, `"images/logo.png":"AssetLogoPng"` generates `case "images/logo.png": assets.ServeAssetLogoPng(w, r)` in the appropriate per‑length handler.

### 9.4: Per‑Length Handler Generation with Frequency‑Ordered Switch Cases

**Intent:** Go's `switch` statement evaluates cases in top-to-bottom order. For string comparisons, the compiler generates linear comparison code. To minimize average comparison count, routes with higher estimated access frequency must appear earlier in the switch statement. This optimization reduces the average number of string comparisons for frequently accessed routes, directly improving hot‑path performance.

**Purpose:** The router must process requests with O(1) complexity. While the dispatch array provides O(1) lookup for path length, the per‑length switch statement still performs linear search. By ordering cases by estimated frequency, the average comparison count is minimized proportionally to actual traffic patterns.

**Rationale:** Static web assets exhibit predictable access patterns. Common assets (index files, CSS, JavaScript, favicon) receive significantly higher traffic than peripheral assets (deep directory content, large files, obscure file types). The generator can estimate these frequencies using simple heuristics without external data, enabling compile‑time optimization.

**Motivation:** For a switch with N cases, average comparisons range from 1 (first case matches) to N (default). If the top K cases cover the majority of traffic, average comparisons approach K rather than N/2. For example, if 5 cases cover 80% of traffic, average comparisons drop from N/2 to approximately 1-3 for most requests.

**Algorithm:** For each per‑length handler `getLenL`, the generator:

1. **Collects all routes** of byte length `L` (canonical paths, shortcut paths, duplicate paths).
2. **Computes a frequency score** for each route using deterministic heuristics.
3. **Sorts routes by descending score** within the per‑length group.
4. **Emits switch cases** in sorted order, highest‑frequency routes first.

**Frequency Estimation Heuristics (applied per route of length L) - Scoring Implementation:**

```go
func estimateFrequencyScore(path string, isEmbedEligible bool) int {
    // Root index (empty path or "index.html")
    if path == "" || path == "index.html" {
        return 1000
    }
    
    // Root favicon.ico favicon.svg favicon.png… 
    if strings.Contains(path, "favicon.") {
        return 900
    }
    
    // Any CSS file
    if strings.HasSuffix(path, ".css") {
        return 800
    }
    
    // Any JS file
    if strings.HasSuffix(path, ".js") {
        return 600
    }
    
    // Other Index files
    if strings.Contains(path, "index.html") {
        return 500
    }
        
    // any logo image 
    if strings.Contains(path, "logo.") {
        return 400
    }
        
    score := 0
    
    // Heuristic 1: Embed-eligible assets
    if isEmbedEligible {
        score += 200
    }

    // Heuristic 2: Long path penalty
    score -= 5*len(path)
    
    // Heuristic 3: Shallow directory depth
    score -= 30*strings.Count(path, "/")
    
    // Heuristic 4: Low-traffic extensions
    lowTraffic := []string{".map", ".zip", ".pdf", ".doc", ".xls", ".tar"}
    for _, ext := range lowTraffic {
        if strings.Contains(path, ext) {
            score -= 100
            break
        }
    }
    
    return score
}
```

**Handler Generation Example:**

For `L = 12` (12‑byte paths), all routes are 12 bytes long. The generator computes scores and orders cases accordingly:

```go
func getLen12(w http.ResponseWriter, r *http.Request) {
    const L = 12
    pathNoLeadingSlash := r.URL.Path[1:] // raw path without leading '/'
    truncatedPath := pathNoLeadingSlash[:L] // 12-byte truncated path

    // Routes ordered by descending frequency score
    switch truncatedPath {
    case "css/main.css": // Score: 800
        assets.ServeAssetMainCss(w, r)
    case "js/app.v1.js": // Score: 600
        assets.ServeAssetAppJs(w, r)
    case "about/vision": // Score: 500
        assets.ServeAssetAboutVisionIndexHtml(w, r)
    case "img/logo.png": // Score: 400
        assets.ServeAssetLogoPng(w, r)
    case "files/01.zip": // Score: -150
        serveAssetFiles01Zip(w, r)
    // ... all other cases ordered by score ...
    default:
        dispatch[L-1](w, r) // Fallback to nearest prefix
    }
}
```

**Safety guarantee:** The dispatch array logic ensures that `getLenL` is only invoked for paths where `len(pathNoLeadingSlash) >= L`, so the truncation `pathNoLeadingSlash[:L]` is always safe.

**Performance Impact:** For a typical web application with 50 routes at length 12:
- Without frequency ordering: average comparisons = 25 (random distribution)
- With frequency ordering: average comparisons = 2-3 (top routes cover 80% traffic)
- **Performance gain:** ~10x reduction in average string comparisons for frequently accessed routes.

### 9.5: Large Asset Handler Example

For large assets (embed‑ineligible), the handler function is generated in the main router file:

```go
func serveAssetDownloadsFile(w http.ResponseWriter, r *http.Request) {
    // No pre‑computed headers – http.ServeFile handles everything
    http.ServeFile(w, r, "www/downloads/file.zip")
}
```

The path is relative to the binary's working directory, where the large asset is symbolically linked from input via the `www/` tree.

### 9.6: No‑Nil Guarantee

For each length `L` (`0 ≤ L ≤ MaxLen`):
- If no canonical, shortcut, or duplicate path has length L, locate nearest‑shorter index K where `dispatch[K] != nil` (`K < L`) and set `dispatch[L] = dispatch[K]`.
- If no shorter K exists, set `dispatch[L] = http.NotFound`.
- For `dispatch[0]`: if no canonical or shortcut path matches length 0, set `dispatch[0] = http.NotFound`.

This guarantees **no nil entries**.

### 9.7: Top‑Level Router

```go
func (s *server) router(w http.ResponseWriter, r *http.Request) {
    idx := len(r.URL.Path) - 1 // length without leading '/'
    if idx < len(s.dispatch) {
        s.dispatch[idx](w, r)
        return
    }
    // Path longer than any known route → Nearest‑shorter‑prefix
    s.dispatch[len(s.dispatch)-1](w, r)
}
```

**Prefix matching applies to all paths** (both embed‑eligible and large asset paths). When a request path is longer than any registered path, the router searches backwards for the largest route path that is a prefix of the request path. This fallback applies uniformly to all asset types.

---

## 10. Runtime Architecture

### 10.1: Core server Struct

```go
type server struct {
    logger   *slog.Logger
    dispatch [MaxLen+1]func(http.ResponseWriter, *http.Request)
    tlsCfg   *tls.Config
}
```

### 10.2: Request Handling Flow

1. **Router** – `dispatch[len(r.URL.Path)-1](w, r)` (array lookup).
2. **Per‑length handler** – `switch` over truncated path slice (no allocation).
3. **Asset handler** – For embed‑eligible: calls `assets.ServeAssetXXX(w, r)` which writes pre‑computed header literal (`Write` once) and, if method ≠ `HEAD`, writes body literal (`Write` once). For large assets: calls local `serveAssetXXX(w, r)` using `http.ServeFile(w, r, "www/...")`. Duplicates call canonical handler from `assets`.
4. **Conditional GET** – Single `if` checking both `If‑None‑match` and `If‑Modified‑Since`.
5. **Large assets** – Served via per‑request `http.ServeFile` calls from `www/` tree.

**Hot path:** Contains **one array bounds check**, **one `switch`**, **one conditional‑GET `if`**.

### 10.3: Protocol Listeners

| Listener | Protocols | Implementation |
|----------|-----------|----------------|
| Plain HTTP (`--http`) | HTTP/1.1 + h2c Upgrade | `golang.org/x/net/http2` (via `http2.ConfigureServer` or `http2.NewHandler`) |
| HTTPS (`--https`) | HTTP/2 (ALPN) + HTTP/3 (QUIC) | `tls.Config{MinVersion: tls.VersionTLS13, NextProtos: []string{"h3","h2","http/1.1"}}` + `github.com/quic-go/quic-go/http3` for QUIC listener |
| Admin (`--admin`) | Plain HTTP only | Simple `http.Server` with `/healthz`, `/readyz`, and `/version` handlers |

HTTP/3 requires UDP listener on the same port as HTTPS TCP listener.

### 10.4: Graceful Shutdown & Signal Handling

```go
sigc := make(chan os.Signal, 1)
signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

go func() {
    for {
        sig := <-sigc
        switch sig {
        case syscall.SIGHUP:
            s.cycleLogLevel() // WARN → INFO → DEBUG → WARN
        default:
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            var wg sync.WaitGroup
            wg.Add(1)
            go func() {
                defer wg.Done()
                _ = httpSrv.Shutdown(ctx)
            }()
            if adminSrv != nil {
                wg.Add(1)
                go func() {
                    defer wg.Done()
                    _ = adminSrv.Shutdown(ctx)
                }()
            }
            wg.Wait()
            s.logger.Info("shutdown complete")
            return
        }
    }
}()
```

**Signal handling details:**
- `SIGINT`: Triggers graceful shutdown with 5‑second timeout.
- `SIGTERM`: Triggers graceful shutdown with 5‑second timeout.
- `SIGHUP`: Cycles log level (WARN → INFO → DEBUG → WARN).

---

## 11. Security Considerations

| Threat | Mitigation |
|--------|------------|
| **XSS** | CSP header for HTML assets (FR‑7). |
| **MIME sniffing** | `X‑Content‑Type‑Options: nosniff` for known MIME types. |
| **MITM** | TLS 1.3 enforced; self‑signed RSA 2048‑bit cert generated at start‑up (FR‑9). |
| **Path traversal** | Router works on raw request path slices; embed‑eligible assets never touch filesystem; large assets served via `http.ServeFile` from `www/` with no path traversal. |
| **Slow‑loris / DoS** | Server timeouts (NFR‑7). |
| **Blocked ports** | Runtime check against Appendix A (FR‑22). |
| **Cache poisoning** | Cache filenames derived from deterministic `LEVEL`/`QUALITY` and `imohash`; collisions impossible. |
| **Admin exposure** | Admin server runs on user‑provided address; defaults to localhost. |
| **CGO dependency** | Generator requires CGO tool‑chain (FR‑39). Generated server is pure Go. |
| **No runtime compression** | All variants pre‑computed; server never calls Brotli/AVIF/WebP (FR‑44). |
| **Asset isolation** | Embed‑eligible assets isolated in `assets/`; large assets isolated in `www/`. |

---

## 12. Error Handling

| Exit code | Meaning |
|-----------|---------|
| **1** | Invalid command‑line usage (missing positional arguments, unknown flag, etc.). |
| **2** | Generator‑internal error (I/O failure, validation error, blocked port, etc.). |
| **3** | Test‑suite failure (`--tests`), emitted as **E079**. |
| **10** | **E010** – Blocked‑port violation (generated server only). |
| **87** | **E087** – Failure to create symbolic/hard link, or cross‑filesystem hard‑link attempt (FR‑23). |

All error messages are prefixed with the corresponding error code (`E001`, `E087`, …) and emitted via the logger at `ERROR` level.

---

## 13. Deployment Guidance

### 13.1 Generating the Server

**Usage:**
```bash
flashbuilder <input> <output> [flags]
```

**Typical production run:**
```bash
flashbuilder ./website ./generated \
    --embed-budget=4G \
    --brotli=11 \
    --avif=70 \
    --webp=70 \
    --csp="default-src 'self'" \
    -v
```

**Result:**
- output contains:
  - `assets/` sub‑directory with single `embed.go` and all embed‑eligible assets as links
  - `www/` tree with large assets (mirrors input structure)
  - Main router file
  - Go module files (`go.mod`, `go.sum`)
  - Compiled binary `flash`
- Link structure:
  - **Symbolic links in `assets/`** → point to original files in input (relative paths)
  - **Hard links in `assets/`** → point to variant files in Cache directory (absolute paths)
  - **Symbolic links in `www/`** → point to original files in input (relative paths, preserving directory structure)
  - **Duplicate paths** → resolved by router, no files created
- Original input tree untouched.

### 13.2 Running the Generated Server

**Usage:**
```bash
./generated/flash \
    --http=0.0.0.0:8080 \
    --https=0.0.0.0:8443 \
    --admin=127.0.0.1:8081 \
    -v
```

**Key points:**
- TLS 1.3 always used; self‑signed certificate regenerated on each start‑up.
- HTTP/2 (via ALPN) and HTTP/3 (via QUIC) automatically advertised.
- `Alt‑Svc` header advertises HTTP/3 on HTTPS port.
- Server refuses to bind to any port listed in Appendix A, exiting with **E010**.
- Embed‑eligible assets served from `assets` package via `//go:embed`.
- Large assets served from `www/` tree via `http.ServeFile`.

---

## 14. Testing Strategy

| Test type | Description |
|-----------|-------------|
| **Unit** | Identifier sanitisation, MIME detection, header rendering, embed‑budget logic, directory creation logic, link path calculation. |
| **Integration** | Spins up generated server on random ports, exercises all protocol listeners, verifies correct response headers, variant selection, `Alt‑Svc`/`HSTS`. |
| **Fuzz** | `FuzzIdentifierCollision` (random path strings) and `FuzzRouterPath` (random request paths) – ensure no panic, correct 404 handling. |
| **Benchmark** | Measures per‑request latency, allocations, throughput for embed‑eligible vs large assets. Benchmarks must report **zero heap allocations** for the hot path using `go test -bench=. -benchmem`. |
| **Error‑path** | Tests for all error codes (E001, E010, E087, E079) including CGO compilation failures, cache I/O errors, embed budget calculation errors, router generation errors. |
| **Protocol** | Integration tests for HTTP/2 and HTTP/3 protocols, including h2c upgrade, ALPN negotiation, and QUIC listener functionality. |
| **Link‑path** | Edge cases for `filepath.Rel` calculations including deep directory structures, same-directory links, and cross-directory links. |

Any failure aborts generation with exit code 3 (`E079`).

---

## 15. Appendices

### A. Blocked‑Port Lists

```
1, 7, 9, 11, 13, 15, 17, 19, 20, 21, 22, 23, 25,
37, 42, 43, 53, 69, 77, 79, 87, 95, 101, 102, 103,
104, 109, 110, 111, 113, 115, 117, 119, 123, 135,
137, 139, 143, 161, 179, 389, 427, 465, 512, 513,
514, 515, 526, 530, 531, 532, 540, 548, 554, 556,
563, 587, 601, 636, 989, 990, 993, 995, 1719,
1720, 1723, 2049, 3659, 4045, 4190, 5060,
5061, 6000, 6566, 6665, 6666, 6667, 6668,
6669, 6679, 6697, 10080
```

### B. Sample `--dry‑run` Summary

```
=== flashbuilder dry‑run summary ===
Source directory (input):    ./website
Output directory (output):    ./generated
MaxLen: 27
Total assets discovered:    842
  – Embed‑eligible assets:  542 (selected by greedy size‑budget)
    – Consolidated in assets/ directory
    – Single embed.go file generated
  – Large assets:           300 (served from www/ tree)
  – Duplicate assets:       12 (router entries only, no files)
Variants kept:
  – Brotli:   112 files (78 MiB total)
  – AVIF:      47 files (31 MiB total)
  – WebP:      47 files (29 MiB total)
Header pre‑computation:       completed
Cache directory: modified (210 reads, 67 writes, 0 evictions) as per normal operation.
Output structure:
  - assets/        → embed.go + all embed‑eligible links
  - www/           → large asset tree (mirrors input)
Dry‑run completed successfully – only the cache directory may have been changed.
```

### C. Sample Output Tree Structure

```
output/
├── assets/
│   ├── embed.go                  // single file with all embed‑eligible assets
│   ├── AssetAboutIndexHtml.html → (symlink) → ../input/about/index.html
│   ├── AssetMainCssCss           → (symlink) → ../input/css/main.css
│   ├── AssetLogoPngAvif          → (hard link) → $FLASHBUILDER_CACHE_DIR/.../hash.avif
│   └── ...                       // all embed‑eligible assets
├── www/
│   ├── images/
│   │   └── gallery/
│   │       └── large-photo.jpg   → (symlink) → ../../../input/images/gallery/large-photo.jpg
│   └── downloads/
│       └── large-file.zip        → (symlink) → ../../../input/downloads/large-file.zip
├── main.go                       // router file with dispatch array
├── go.mod
├── go.sum
└── flash                      // compiled binary
```

### D. Error‑Code Table

| Code | Meaning |
|------|---------|
| **E001** | I/O error while reading an asset (FR‑1). |
| **E010** | Blocked‑port violation (FR‑22). |
| **E025** | Invalid compression/quality level (FR‑5/FR‑6). |
| **E030** | Dry‑run permission validation failure (FR‑17). |
| **E087** | Failure to create symbolic/hard link, or cross‑filesystem hard‑link attempt (FR‑23). |
| **E099** | Generic internal error (fallback). |
| **E079** | Test‑suite failure (`--tests`). |

### E. Flag → Environment‑Variable Mapping

| Flag (CLI) | Environment variable |
|------------|----------------------|
| `--embed-budget` | `FLASHBUILDER_EMBED_BUDGET` |
| `--brotli` | `FLASHBUILDER_BROTLI` |
| `--avif` | `FLASHBUILDER_AVIF` |
| `--webp` | `FLASHBUILDER_WEBP` |
| `--csp` | `FLASHBUILDER_CSP` |
| `-v` (counter) | `FLASHBUILDER_LOG_LEVEL` (or `FLASH_LOG_LEVEL` for generated server) |
| `--dry-run` | `FLASHBUILDER_DRY_RUN` |
| `--tests` | `FLASHBUILDER_TESTS` |
| `--cache-max` | `FLASHBUILDER_CACHE_MAX` |
| `--cache-dir` | `FLASHBUILDER_CACHE_DIR` |
| `--version` | — (no env override) |
| `--help` | — (no env override) |
| *generated‑server flags* | `FLASH_<UPPERCASE_FLAG>` (e.g., `FLASH_HTTP`) |

---

## 16. Revision History

| Version | Date | Changes |
|---------|------|---------|
| **18.0.0** | 2026‑10‑01 | • Improved machine parseability for downstream Go code generators.<br>• Consolidated all error codes in Appendix D.<br>• Clarified frequency estimation algorithm with complete scoring heuristics table.<br>• Added explicit Intent, Purpose, Rationale, and Motivation sections for router optimization.<br>• Provided complete Go code examples for frequency scoring function.<br>• Ensured consistent terminology throughout specification.<br>• Explicit defaults for all CLI flags.<br>• Complete data structure definitions.<br>• Numbered workflow steps consistently.<br>• Provided code examples for complex logic (router, handlers, embed.go structure). |
| 17.0.0 | 2026‑09‑20 | • Added frequency‑ordered switch case optimization for router performance.<br>• Documented performance impact: ~10x reduction in average string comparisons.<br>• Provided scoring algorithm with heuristics table. |
| 16.0.0 | 2026‑09‑15 | • Clarified MIME detection order, CSP defaults, duplicate handling, HTTP/3 requirements.<br>• Added benchmark verification, import validation, health‑check semantics, signal handling. |
| 15.0.0 | 2026‑09‑01 | Initial machine‑parseable specification format. |

---

**End of Specification**
