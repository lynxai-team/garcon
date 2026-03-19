# Go Specification for `flashbuilder` - Ultra-Fast Static Asset HTTP Server Generator

## 1. EXECUTIVE SUMMARY

This specification defines `flashbuilder`, a Go code generator that produces `flash` - an ultra-high-performance static asset HTTP server. The generated server must achieve the absolute minimum computational overhead per request:

**Performance Contract (Hot Path):**
- **Maximum 1 array bounds check** (dispatch array indexed by path length)
- **Maximum 1 switch statement** (per-length path routing)
- **Maximum 1 conditional GET check** (ETag/Last-Modified validation)
- **Zero heap allocations** for embed-eligible assets
- **Zero reflection** in any request processing path
- **Zero interface dispatch** in hot path
- **Zero dynamic map lookups** in hot path
- **Pre-computed `[]byte` constants** for all headers
- **Pre-computed `[]byte` constants** for all embed-eligible content

---

## 2. ARCHITECTURAL PRINCIPLES

### 2.1 Compile-Time Pre-computation Doctrine

**Rule 1:** Any value computable at generation time SHALL be emitted as a compile-time constant.

**Rule 2:** All HTTP response headers SHALL be pre-computed as `[]byte` literals.

**Rule 3:** All embed-eligible content SHALL be emitted via `//go:embed` directives.

**Rule 4:** All routing decisions SHALL be pre-computed into length-indexed dispatch arrays.

**Rule 5:** All string operations SHALL be eliminated from hot path via pre-computation.

### 2.2 Memory Layout Doctrine

**Rule 6:** All arrays SHALL be sized at compile time with known bounds.

**Rule 7:** No dynamic allocation SHALL occur in request handling.

**Rule 8:** All string-to-byce conversions SHALL be pre-computed.

**Rule 9:** All path segments SHALL be stored as pre-hashed values if comparison is required.

### 2.3 Branch Elimination Doctrine

**Rule 10:** All conditional logic SHALL be collapsed into switch statements.

**Rule 11:** Switch statements SHALL use compile-time known case values.

**Rule 12:** Fall-through SHALL be preferred over nested conditionals.

**Rule 13:** Dispatch tables SHALL replace if-else chains.

---

## 3. PERFORMANCE TARGETS

### 3.1 Latency Targets (per request on modern hardware)

| Asset Type | Target Latency | Allocation Target |
|------------|---------------|-------------------|
| Embed-eligible (hot path) | ≤100ns | 0 bytes |
| Cached variant (warm path) | ≤500ns | ≤16 bytes |
| Large file (cold path) | ≤1μs | ≤64 bytes |

### 3.2 Throughput Targets

| Protocol | Target RPS | Target Connections |
|----------|-----------|-------------------|
| HTTP/1.1 keep-alive | ≥300K rps | 10K concurrent |
| HTTP/2 | ≥500K rps | 100K concurrent streams |
| HTTP/3 (QUIC) | ≥400K rps | 100K concurrent streams |

### 3.3 Code Generation Constraints

| Constraint | Value | Rationale |
|------------|-------|-----------|
| Generated file size | Unlimited | Performance over binary size |
| Generated Go complexity | Unlimited | Performance over compilation time |
| Generated struct count | As needed | Each asset gets dedicated handler |
| Generated function count | As needed | Each path length gets dedicated handler |

---

## 4. CLI SPECIFICATION

### 4.1 Generator CLI (`flashbuilder`)

**Usage:** `flashbuilder <input> <output> [flags]`

**Positional Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `input` | `string` | Path to asset tree (read-only, never modified) |
| `output` | `string` | Destination for generated files. Must not equal input. Created if missing. |

**Flags:**

| Flag | Type | Go Type | Default | Description |
|------|------|---------|---------|-------------|
| `--embed-budget` | size | custom | `0` | Cumulative size limit for embed-eligible assets. `0` means unlimited. `1` effectively disables embedding. Suffixes: `K`=1024, `M`=1024², `G`=1024³, `T`=1024⁴. |
| `--brotli` | int | `kong.Int` | `11` | Brotli compression level (0-11). Negative disables. |
| `--avif` | int | `kong.Int` | `50` | AVIF quality (0-100). Negative disables. |
| `--webp` | int | `kong.Int` | `50` | WebP quality (0-100). Negative disables. |
| `--csp` | string | `kong.String` | `"default-src 'self'"` | CSP header for HTML. Empty string disables. Unset uses default. |
| `-v` | counter | `kong.Counter` | `0` | Log level: 0=WARN, 1=INFO, ≥2=DEBUG. |
| `--dry-run` | bool | `kong.Bool` | `false` | Simulate without writing output. |
| `--tests` | bool | `kong.Bool` | `false` | Run `go test ./... -race -vet=all` after generation. |
| `--cache-max` | size | custom | `5 GiB` | Cache directory upper bound. |
| `--cache-dir` | path | `kong.Path` | `CACHE_DIR` | Cache directory. Must be same filesystem as output. Default follows XDG: `$XDG_CACHE_HOME/flashbuilder` or `$HOME/.cache/flashbuilder`. |
| `--version` | bool | `kong.Bool` | `false` | Print version and exit. |
| `--help` | bool | `kong.Bool` | `false` | Show usage. |

`CACHE_DIR` follows XDG Base Directory Specification: `$XDG_CACHE_HOME/flashbuilder` if `$XDG_CACHE_HOME` is set; otherwise `$HOME/.cache/flashbuilder` if `$HOME` is set; otherwise `./.cache` (current directory). The Cache directory must be on the same filesystem as output for hard-links.
**Environment Variable Overrides:**

| Flag | Environment Variable |
|------|----------------------|
| `--embed-budget` | `FLASHBUILDER_EMBED_BUDGET` |
| `--brotli` | `FLASHBUILDER_BROTLI` |
| `--avif` | `FLASHBUILDER_AVIF` |
| `--webp` | `FLASHBUILDER_WEBP` |
| `--csp` | `FLASHBUILDER_CSP` |
| `-v` | `FLASHBUILDER_LOG_LEVEL` |
| `--dry-run` | `FLASHBUILDER_DRY_RUN` |
| `--tests` | `FLASHBUILDER_TESTS` |
| `--cache-max` | `FLASHBUILDER_CACHE_MAX` |
| `--cache-dir` | `FLASHBUILDER_CACHE_DIR` |

**Size Parsing Grammar:**
```
<size>   ::= <number> [ <suffix> ]
<suffix> ::= "K" | "M" | "G" | "T" | "KiB" | "MiB" | "GiB" | "TiB"
<number> ::= [0-9]+
```
Multiplier: K=1024¹, M=1024², G=1024³, T=1024⁴

### 4.2 Generated-Server CLI (`flash`)

**Usage:** `flash [flags]`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--http` | string | `localhost:8080` | Plain-HTTP bind address. `none` disables. Supports HTTP/1.1 + h2c Upgrade. |
| `--https` | string | `localhost:8443` | TLS 1.3 bind address. `none` disables. Enables HTTP/2 via ALPN and HTTP/3 via QUIC. |
| `--admin` | string | `localhost:8081` | Admin listener. `none` disables. |
| `-v` | counter | `0` | Log level: 0=WARN, 1=INFO, ≥2=DEBUG. |
| `--help` | bool | `false` | Show usage. |

**Environment Variable Overrides:**

| Flag | Environment Variable |
|------|----------------------|
| `--http` | `FLASH_HTTP` |
| `--https` | `FLASH_HTTPS` |
| `--admin` | `FLASH_ADMIN` |
| `-v` | `FLASH_LOG_LEVEL` |

**All flag parsing MUST use `kong` exclusively. No `os.Getenv` calls permitted.**

---

## 5. DATA STRUCTURES

### 5.1 Asset Struct

The core data structure representing each discovered file:

```go
type Asset struct {
    RelPath        string        // POSIX-style path relative to input: "css/main.css"
    AbsPath        string        // Absolute input path (for symlink creation)
    Size           int64         // Raw file size in bytes
    ModTime        time.Time     // Modification time (UTC)
    MIME           string        // Detected MIME type
    ImoHash        uint64        // imohash of raw content
    IsDuplicate    bool          // True if another asset shares identical content
    CanonicalID    string        // Identifier of canonical asset (if duplicate)
    EmbedEligible  bool          // True if selected by embed-budget
    Variants       []Variant     // Kept variants (max one per type)
    HeaderLiteral  []byte        // Pre-computed header block (embed-eligible only)
    Identifier     string        // Exported Go identifier (prefixed with "Asset")
    Filename       string        // Filename in assets/: "AssetMainCss.css"
    FrequencyScore int           // Estimated request traffic frequency
}
```

### 5.2 Variant Struct

```go
type Variant struct {
    Type          VariantType   // Brotli, AVIF, WebP
    Size          int64         // Variant file size in bytes
    HeaderLiteral []byte        // Pre-computed header block
    Identifier    string        // Exported Go identifier
    Extension     string        // File extension: "br", "avif", "webp"
    CachePath     string        // Absolute path in cache directory
}
```

### 5.3 VariantType Enum

```go
type VariantType int

const (
    VariantBrotli VariantType = iota
    VariantAVIF
    VariantWebP
)
```

### 5.4 Router Maps

```go
var (
    canonicalPaths  map[string]string // canonical path → identifier
    shortcutPaths   map[string]string // shortcut path → identifier
    duplicatePaths  map[string]string // duplicate path → canonical identifier
)
```

### 5.5 Generated Server State

```go
type Server struct {
    logger   *slog.Logger
    dispatch [MaxLen+1]func(http.ResponseWriter, *http.Request)
    tlsCfg   *tls.Config
}
```

### 5.6 Constants

```go
const MaxLen = <computed_at_generation>  // Maximum path length in asset set
```

---

## 6. ALGORITHMS

### 6.1 Asset Discovery Algorithm

```
FUNCTION discoverAssets(input):
    assets = []
    for each file in walk(input, recursive=true, follow_symlinks_to_files=true):
        if file is socket or device or FIFO:
            continue
        if file is symlink to directory:
            continue (don't follow)
        if file is regular file or symlink to regular file:
            asset = Asset{
                RelPath:  relative_path_from_input_dir,
                AbsPath:  absolute_path,
                Size:     file_size,
                ModTime:  file_modtime,
                MIME:     detect_mime(file),
                ImoHash:  imohash(file),
            }
            assets.append(asset)
    return assets
```

**Implementation Requirements:**
- Use `filepath.Walk()` with symlink following
- Skip `os.ModeSocket | os.ModeDevice | os.ModeNamedPipe`
- Record `os.FileInfo.Size()` and `os.FileInfo.ModTime()`
- Normalize paths to POSIX-style forward slashes
- All paths MUST be relative to input directory without leading `/`

### 6.2 MIME Detection Algorithm

**Priority Order:**
1. Extension lookup (case-insensitive)
2. Content sniffing (first 512 bytes via `http.DetectContentType`)
3. Fallback to `application/octet-stream`

```
FUNCTION detect_mime(file):
    extension = extract_extension(file)
    if extension == "":
        first_512_bytes = read_first_512_bytes(file)
        mime = http.DetectContentType(first_512_bytes)
        if mime == "":
            return "application/octet-stream"
        return mime
    else:
        mime = mime.TypeByExtension(extension)
        if mime == "":
            first_512_bytes = read_first_512_bytes(file)
            mime = http.DetectContentType(first_512_bytes)
            if mime == "":
                return "application/octet-stream"
            return mime
        return mime
```

**HTML Detection Rule:**
- MIME starts with `"text/html"` → HTML asset
- Apply CSP header only to HTML

### 6.3 Imohash Algorithm

```go
import "github.com/kalafut/imohash"

func computeImoHash(file io.Reader) uint64 {
    hasher := imohash.New()
    hasher.Write(readAllBytes(file))
    return hasher.Sum64()
}
```

**Note:** Imohash provides fast content-addressable hashing for deduplication.

### 6.4 Deduplication Algorithm (BEFORE Budget Calculation)

```
FUNCTION deduplicate(assets):
    hashMap = map[uint64][]Asset{}
    
    for each asset in assets:
        key = asset.ImoHash
        if hashMap[key] == nil:
            hashMap[key] = []Asset{asset}
        else:
            if bytes_equal(asset.content, hashMap[key][0].content):
                asset.IsDuplicate = true
                asset.CanonicalID = hashMap[key][0].Identifier
            else:
                hashMap[key] = append(hashMap[key], asset)
    
    for each key in hashMap:
        if len(hashMap[key]) > 1:
            canonical = hashMap[key][0]
            for each duplicate in hashMap[key][1:]:
                duplicate.IsDuplicate = true
                duplicate.CanonicalID = canonical.Identifier
    
    return hashMap
```

**CRITICAL:** Deduplication occurs BEFORE budget calculation. This is essential for correct embed budget allocation.

### 6.5 Variant Keep Condition

```
FUNCTION should_keep_variant(variant_size, original_size):
    // Keep if EITHER condition satisfied:
    // (variant_size < original_size - 3 KiB) OR (variant_size < 0.9 * original_size)
    
    if variant_size < (original_size - 3*1024):
        return true
    if variant_size < int64(0.9 * float64(original_size)):
        return true
    return false
```

**Brotli Conditions:**
- Enabled when `--brotli ≥ 0`
- Applies ONLY to assets ≥ 2 KiB
- MIME is NOT `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/avif`
- Keep `.br` variant iff keep condition satisfied

**AVIF/WebP Conditions:**
- Enabled when `--avif ≥ 0` and/or `--webp ≥ 0`
- Applies ONLY to `image/png` and `image/jpeg`
- Keep smallest variant iff keep condition satisfied

**Smallest variant wins when multiple satisfy. Only one variant per asset.**

### 6.6 Embed Budget Allocation Algorithm

```
FUNCTION allocate_budget(assets, budget):
    if budget == 0:
        budget = unlimited
    
    if budget == 1:
        for each asset in assets:
            asset.EmbedEligible = false
        return
    
    // Sort by size ascending (smallest first)
    sort.Assets.By.Size.Ascending(assets)
    
    cumulative = 0
    for each asset in assets:
        if asset.IsDuplicate:
            continue
        if budget > 0 and (cumulative + asset.Size > budget):
            break
        asset.EmbedEligible = true
        cumulative += asset.Size
```

### 6.7 Identifier Generation Algorithm

```go
func generateIdentifier(relPath string) string {
    segments := strings.Split(relPath, "/")
    var parts []string
    
    for _, segment := range segments {
        for _, part := range strings.Split(segment, ".") {
            if part == "" {
                continue
            }
            if len(part) > 0 {
                part = strings.ToUpper(string(part[0])) + part[1:]
            }
            part = strings.Trim(part, "!#$%&'*+,-./:;<=>?@^_`|~")
            if part != "" {
                parts = append(parts, part)
            }
        }
    }
    
    identifier := "Asset" + strings.Join(parts, "")
    
    if exists(identifier):
        for i := 0; ; i++ {
            candidate := fmt.Sprintf("%s_%d", identifier, i)
            if !exists(candidate):
                return candidate
        }
    }
    
    return identifier
}
```

**Collision Resolution:** Maintain global identifier map, append `_N` suffix for duplicates.

### 6.8 Frequency Estimation Algorithm

```go
func estimateFrequencyScore(path string, isEmbed bool) int {
    if path == "" || path == "index.html":
        return 1000
    if strings.Contains(path, "favicon."):
        return 900
    if strings.HasSuffix(path, ".css"):
        return 800
    if strings.HasSuffix(path, ".js"):
        return 600
    if strings.Contains(path, "index.html"):
        return 500
    if strings.Contains(path, "logo."):
        return 400
    
    score := 0
    if isEmbed {
        score += 200
    }
    score -= 5 * len(path)
    score -= 30 * strings.Count(path, "/")
    
    lowTraffic := []string{".map", ".zip", ".pdf", ".doc", ".xls", ".tar"}
    for _, ext := range lowTraffic {
        if strings.HasSuffix(path, ext) {
            score -= 100
            break
        }
    }
    return score
}
```

### 6.9 Dispatch Array Population Algorithm

```
FUNCTION build_dispatch_array(all_paths):
    MaxLen = compute_maxlen(all_paths)
    dispatch = make([]func(http.ResponseWriter, *http.Request), MaxLen+1)
    
    if has_root_index():
        dispatch[0] = serve_root_index()
    else:
        dispatch[0] = http.NotFound
    
    for L = 1 to L = MaxLen+1:
        routes = collect_routes_of_length(L)
        if len(routes) > 0:
            sortRoutesByFrequency(routes)
            dispatch[L] = generate_per_length_handler(L, routes)
        else:
            K = find_nearest_valid_dispatch(dispatch, L-1)
            if K >= 0:
                dispatch[L] = dispatch[K]
            else:
                dispatch[L] = http.NotFound

    return dispatch
```

**find_nearest_valid_dispatch:**
```go
func find_nearest_valid_dispatch(dispatch []http.HandlerFunc, maxIndex int) int {
    for i := maxIndex; i >= 0; i-- {
        if dispatch[i] != nil && dispatch[i] != http.NotFound {
            return i
        }
    }
    return -1
}
```

### 6.10 Router Fallback Algorithm

```go
func fallback_long_path(w http.ResponseWriter, r *http.Request, path string) {
    for L := MaxLen; L >= 0; L-- {
        if len(path) >= L {
            prefix := path[:L]
            if dispatch[L] != nil && dispatch[L] != http.NotFound {
                if is_prefix_match(prefix, known_routes_of_length_L):
                    dispatch[L](w, r)
                    return
                }
            }
        }
    }
    http.NotFound(w, r)
}
```

### 6.11 Link Creation Algorithm

**Case 1: Canonical embed-eligible with variant:**
- Create hard link in `assets/` as `Asset<Identifier>.<variant-ext>` pointing to variant in cache directory (absolute path)
- Do NOT create symbolic link for original

**Case 2: Canonical embed-eligible without variant:**
- Create symbolic link in `assets/` as `Asset<Identifier>.<extension>` pointing to original in input (relative path via `filepath.Rel`)

**Case 3: Large asset:**
- Create symbolic link in `www/<relpath>/<filename>` pointing to original in input (relative path)
- Create intermediate directories, prune empty directories after

**Case 4: Duplicate asset:**
- No file creation
- Router maps duplicate path to canonical handler

---

## 7. CODE TEMPLATES

### 7.1 embed.go Template

```go
// Package assets contains all embedded assets.
// Generated by flashbuilder - DO NOT EDIT
package assets

import (
    _ "embed"
    "net/http"
    "github.com/alecthomas/kong"
)

//go:embed AssetAboutIndexHtml.html
var AssetAboutIndexHtml []byte

//go:embed AssetMainCss.css
var AssetMainCss []byte

// ... all embedded assets ...

var AssetAboutIndexHtmlHeader []byte = []byte("Content-Type: text/html; charset=utf-8\r\nContent-Security-Policy: default-src 'self'\r\nCache-Control: public, max-age=31536000, immutable\r\nContent-Length: <SIZE>\r\n\r\n")

var AssetMainCssHeader []byte = []byte("Content-Type: text/css; charset=utf-8\r\nCache-Control: public, max-age=31536000, immutable\r\nContent-Length: <SIZE>\r\n\r\n")

// ... all header literals ...

func ServeAssetAboutIndexHtml(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write(AssetAboutIndexHtmlHeader)
    if r.Method != http.MethodHead {
        _, _ = w.Write(AssetAboutIndexHtml)
    }
}

func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    _, _ = w.Write(AssetMainCssHeader)
    if r.Method != http.MethodHead {
        _, _ = w.Write(AssetMainCss)
    }
}

// ... all handler functions ...
```

### 7.2 Header Literal Format

For each embed-eligible asset, generate a constant header slice:

```go
var Asset<Identifier>Header []byte = []byte("Content-Type: <MIME>\r\nCache-Control: public, max-age=31536000, immutable\r\nContent-Length: <SIZE>\r\n\r\n")
```

**CSP for HTML:**
- `--csp=""` → no CSP header
- Unset → `"default-src 'self'"`
- Custom value → use that value

**TLS headers:**
- `Strict-Transport-Security: max-age=31536000` on every TLS response
- `Alt-Svc: h3=":<port>"; ma=2592000` on every TLS response

### 7.3 main.go Template

```go
// Package main is the main entry point for flash server.
// Generated by flashbuilder - DO NOT EDIT
package main

import (
    "net/http"
    "net/http/pprof"
    "github.com/alecthomas/kong"
    "golang.org/x/net/http2"
    "github.com/quic-go/quic-go/http3"
    "flash/assets"
)

type Server struct {
    logger   *slog.Logger
    dispatch [MaxLen+1]func(http.ResponseWriter, *http.Request)
    tlsCfg   *tls.Config
}

func getLen0(w http.ResponseWriter, r *http.Request) {
    // Generated switch for length 0
}

// ... up to getLen<MaxLen> ...

func serveAssetDownloadsFile(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, "www/downloads/file.zip")
}

func (s *Server) router(w http.ResponseWriter, r *http.Request) {
    path := r.URL.Path[1:]
    idx := len(path)
    if idx < len(s.dispatch) {
        s.dispatch[idx](w, r)
        return
    }
    s.dispatch[len(s.dispatch)-1](w, r)
}

func main() {
    var cli struct {
        HTTP   string `kong:"--http,default=localhost:8080"`
        HTTPS  string `kong:"--https,default=localhost:8443"`
        Admin  string `kong:"--admin,default=localhost:8081"`
        V      int    `kong:"-v,default=0"`
    }
    kong.Parse(&cli)
    // ... server setup ...
}
```

### 7.4 Per-Length Handler Template

```go
func getLen<LENGTH>(w http.ResponseWriter, r *http.Request) {
    const L = <LENGTH>
    pathNoLeadingSlash := r.URL.Path[1:]
    truncatedPath := pathNoLeadingSlash[:L]
    
    switch truncatedPath {
    case "css/main.css": // Score: 800
        assets.ServeAssetMainCss(w, r)
    case "js/app.v1.js": // Score: 600
        assets.ServeAssetAppJs(w, r)
    // ... all cases ordered by score ...
    default:
        dispatch[L-1](w, r)
    }
}
```

### 7.5 Health Endpoints

```go
func healthzHandler(w http.ResponseWriter, r *http.Request) {
    if isHealthy() {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

func readyzHandler(w http.ResponseWriter, r *http.Request) {
    if isReady() {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
    version := struct {
        GeneratorVersion string `json:"generator_version"`
        BuildTime        string `json:"build_time"`
        AssetCount       int    `json:"asset_count"`
        EmbedSize        int64  `json:"embed_size"`
        VariantCount     int    `json:"variant_count"`
        HTTPBind         string `json:"http_bind"`
        HTTPSBind        string `json:"https_bind"`
    }{
        GeneratorVersion: <GENERATOR_VERSION>,
        BuildTime:        <BUILD_TIME>,
        AssetCount:       <ASSET_COUNT>,
        EmbedSize:        <EMBED_SIZE>,
        VariantCount:     <VARIANT_COUNT>,
        HTTPBind:         <HTTP_BIND>,
        HTTPSBind:        <HTTPS_BIND>,
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(version)
}
```

### 7.6 Graceful Shutdown

```go
func setupGracefulShutdown(httpSrv *http.Server, adminSrv *http.Server, logger *slog.Logger) {
    sigc := make(chan os.Signal, 1)
    signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
    
    go func() {
        for {
            sig := <-sigc
            switch sig {
            case syscall.SIGHUP:
                cycleLogLevel()
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
                logger.Info("shutdown complete")
                return
            }
        }
    }()
}
```

---

## 8. ERROR HANDLING

| Code | Name | Exit | Description |
|------|------|------|-------------|
| E001 | I/O Error | 2 | Error reading asset file |
| E010 | Blocked Port | 10 | Port in blocked list |
| E025 | Invalid Compression | 2 | Invalid Brotli/AVIF/WebP level |
| E030 | Dry-run Permission | 2 | Dry-run validation failed |
| E087 | Link Error | 87 | Cannot create link or cross-filesystem |
| E099 | Generic Error | 2 | Fallback error |
| E079 | Test Failure | 3 | `go test` failed |

**Exit Codes:**
- 0: Success (including dry-run)
- 1: Invalid CLI usage
- 2: Generator internal error
- 3: Test suite failure (E079)
- 10: Blocked port violation (E010)
- 87: Link creation failure (E087)

---

## 9. WORKFLOW STEPS

1. **Validate CLI Arguments:** Parse with `kong`, validate input/output, create output directory, validate cache directory filesystem.
2. **Asset Discovery:** Walk input recursively, follow symlinks to files, ignore sockets/devices/FIFOs, record metadata.
3. **MIME Detection:** Extension lookup → content sniffing → fallback to `application/octet-stream`.
4. **Deduplication:** Build `hashMap`, mark duplicates BEFORE budget calculation.
5. **Variant Generation:** Brotli for non-images ≥ 2 KiB, AVIF/WebP for PNG/JPEG, check cache, generate variants, apply keep condition.
6. **Select Most-Compact Variant:** Keep smallest variant if multiple satisfy condition.
7. **Embed-Budget Allocation:** Sort ascending, greedy accumulation, mark `EmbedEligible`.
8. **Header Pre-computation:** Build `[]byte` header literals for embed-eligible assets.
9. **Identifier Generation:** Sanitize paths, resolve collisions, generate filenames.
10. **Link Creation:** Case 1 (hard link), Case 2 (symbolic link), Case 3 (symbolic link in www/), Case 4 (no creation).
11. **Build Path Maps:** `canonicalPaths`, `shortcutPaths`, `duplicatePaths` with collision detection.
12. **Generate embed.go:** Package, imports, `//go:embed` directives, headers, handlers.
13. **Router Generation:** Compute `MaxLen`, generate dispatch array, per-length handlers, fallback logic.
14. **TLS Configuration:** Self-signed RSA 2048-bit, random 64-bit serial, 10-year validity.
15. **Protocol Listeners:** HTTP/1.1 + h2c, HTTP/2 via ALPN, HTTP/3 via QUIC.
16. **Module Initialization:** `go mod init flash`, `go mod tidy`.
17. **Build:** `go build -trimpath -ldflags="-s -w" -o flash ./...`.
18. **Cache Eviction:** If total cache size > `--cache-max`, evict oldest-modified files.
19. **Testing:** `go test ./... -race -vet=all`, exit with E079 on failure.

---

## 10. DEPENDENCIES

**Generator Dependencies (CGO Required):**
```
github.com/alecthomas/kong
github.com/kalafut/imohash
github.com/google/brotli/go
github.com/vegidio/avif-go
github.com/kolesa-team/go-webp
```

**Generated Server Dependencies (Pure Go):**
```
github.com/alecthomas/kong
golang.org/x/net/http2
github.com/quic-go/quic-go/http3
```

**CGO Requirements:**
- System packages: `libbrotli-dev`, `libavif-dev`, `libwebp-dev`
- Build environment: `CGO_ENABLED=1`
- Linker flags: `-lbrotlienc`, `-lavif`, `-lwebp`

---

## 11. BLOCKED PORTS

The following ports are blocked (Fetch standard, https://fetch.spec.whatwg.org/#port-blocking):

```
1, 7, 9, 11, 13, 15, 17, 19, 20, 21, 22, 23, 25, 37, 42, 43, 53, 69, 77, 79, 87, 95, 101, 102, 103, 104, 109, 110, 111, 113, 115, 117, 119, 123, 135, 137, 139, 143, 161, 179, 389, 427, 465, 512, 513, 514, 515, 526, 530, 531, 532, 540, 548, 554, 556, 563, 587, 601, 636, 989, 990, 993, 995, 1719, 1720, 1723, 2049, 3659, 4045, 4190, 5060, 5061, 6000, 6566, 6665, 6666, 6667, 6668, 6669, 6679, 6697, 10080
```

Generated server must exit with E010 if any bind address uses a blocked port.

---

## 12. SECURITY HEADERS

| Header | When | Value |
|--------|------|-------|
| CSP | HTML assets only | `--csp` flag: empty → disabled, unset → `"default-src 'self'"`, custom → use value |
| HSTS | Every TLS response | `max-age=31536000` |
| Alt-Svc | Every TLS response | `h3=":<port>"; ma=2592000` |
| X-Content-Type-Options | Known MIME | `nosniff` |
| X-Frame-Options | HTML assets | `DENY` |
| Method validation | All requests | Only `GET` and `HEAD` allowed; `405 Method Not Allowed` for others |

---

## 13. TESTING REQUIREMENTS

- **Unit Tests:** Identifier sanitization, MIME detection, header rendering, embed-budget logic, directory creation, link path calculation, frequency estimation.
- **Integration Tests:** Spin up generated server on random ports (`net.Listen(":0")`), exercise all protocol listeners, verify response headers, verify variant selection, verify `Alt-Svc`/`HSTS`.
- **Fuzz Tests:** `FuzzIdentifierCollision` (random path strings), `FuzzRouterPath` (random request paths).
- **Benchmarks:** Per-request latency, allocations, throughput. Verify zero heap allocations for hot path using `go test -bench=. -benchmem`.

---

## 14. OUTPUT STRUCTURE

```
output/
├── assets/
│   ├── embed.go                  // single file with all embedded assets
│   ├── Asset<Identifier>.<ext>   // symbolic links to original or hard links to variants
│   └── ...
├── www/
│   └── <relpath>/<filename>      // symbolic links to large assets
├── main.go                       // router with dispatch array
├── go.mod
├── go.sum
└── flash                         // compiled binary
```

---

## 15. DETERMINISM

- Fixed alphabetical ordering of assets
- Deterministic module name: `flash`
- `go mod init flash` and `go mod tidy`
- TLS certificate randomness is the only source of nondeterminism

---

## 16. PERFORMANCE OPTIMIZATION TECHNIQUES

### 16.1 Eliminate All String Operations in Hot Path

**Before Generation:**
```go
// SLOW: string concatenation at runtime
header := "Content-Type: " + mime + "\r\n" + "Content-Length: " + size + "\r\n"
```

**After Generation:**
```go
// FAST: pre-computed byte slice constant
var AssetMainCssHeader = []byte("Content-Type: text/css\r\nContent-Length: 1234\r\n\r\n")
```

### 16.2 Eliminate Map Lookups in Hot Path

**Before Generation:**
```go
// SLOW: map lookup by path string
handler := pathMap[r.URL.Path]
if handler != nil {
    handler(w, r)
}
```

**After Generation:**
```go
// FAST: array index by path length (compile-time known)
idx := len(path)
if idx < len(dispatch) {
    dispatch[idx](w, r)  // Direct function call
}
```

### 16.3 Eliminate Interface Dispatch in Hot Path

**Before Generation:**
```go
// SLOW: interface dispatch
var handlers map[string]http.HandlerFunc
handler := handlers[path]
```

**After Generation:**
```go
// FAST: direct function pointer array
var dispatch [MaxLen]func(http.ResponseWriter, *http.Request)
dispatch[idx](w, r)
```

### 16.4 Pre-compute All Headers as Constants

**Generated Code:**
```go
// Pre-computed header block
var AssetMainCssHeader = []byte{
    'C', 'o', 'n', 't', 'e', 'n', 't', '-', 'T', 'y', 'p', 'e', ':',
    ' ', 't', 'e', 'x', 't', '/', 'c', 's', 's', '\r', '\n',
    // ... literal bytes for entire header ...
}

// Direct write to ResponseWriter
func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    w.Write(AssetMainCssHeader)  // Zero allocation
    w.Write(AssetMainCss)        // Zero allocation
}
```

### 16.5 Use Path Length as Dispatch Index

**Algorithm:**
1. Compute MaxLen = maximum path length in asset set
2. Create dispatch array of size MaxLen+1
3. Index by `len(r.URL.Path)` - one bounds check only
4. Each dispatch entry is a generated switch statement

### 16.6 Use Switch for Path Matching

**Generated Code:**
```go
func getLen12(w http.ResponseWriter, r *http.Request) {
    // Path length 12, check first 12 characters
    path := r.URL.Path[1:13]  // Skip leading /
    
    switch path {
    case "css/main.css":
        ServeAssetMainCss(w, r)  // Direct call
    case "js/app.v1.js":
        ServeAssetAppJs(w, r)    // Direct call
    // ... all paths of length 12 ...
    default:
        // Fallback to length 11
        getLen11(w, r)
    }
}
```

### 16.7 Embed Small Assets as Byte Slices

**Generated Code:**
```go
//go:embed AssetMainCss.css
var AssetMainCss []byte  // Compiled into binary
```

### 16.8 Use io.Copy for Large Files

**For non-embed assets:**
```go
func serveLargeFile(w http.ResponseWriter, r *http.Request, path string) {
    f, err := os.Open(path)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    defer f.Close()
    
    // Pre-written header
    w.Write(precomputedHeader)
    
    // Zero-copy transfer
    io.Copy(w, f)
}
```

### 16.9 Pre-compute Security Headers

**Generated Code:**
```go
// Pre-computed security header block for HTML
var securityHeaders = []byte("X-Frame-Options: DENY\r\nX-Content-Type-Options: nosniff\r\n")
```

### 16.10 Use HTTP/2 and HTTP/3 Efficiently

**Generated Code:**
```go
// HTTP/2 via h2c (no TLS)
http2Srv := &http2.Server{
    Handler: http.HandlerFunc(s.router),
}

// HTTP/3 via QUIC
http3Srv := &http3.Server{
    TLSConfig: s.tlsCfg,
    Handler:   http.HandlerFunc(s.router),
}
```

---

## 17. HOT PATH ANALYSIS

### 17.1 Request Processing Hot Path

**Zero-Allocation Hot Path:**
```go
// Single bounds check
idx := len(path)
if idx < len(dispatch) {
    // Single function call via array index
    dispatch[idx](w, r)
    return
}
// Fallback for long paths
fallback(w, r)
```

**Per-Length Handler Hot Path:**
```go
func getLen12(w http.ResponseWriter, r *http.Request) {
    // Single switch statement - compile-time known cases
    switch r.URL.Path[1:13] {
    case "css/main.css":
        // Direct function call
        assets.ServeAssetMainCss(w, r)
    case "js/app.v1.js":
        assets.ServeAssetAppJs(w, r)
    // ... all paths of length 12 ...
    default:
        // Single fallback call
        getLen11(w, r)
    }
}
```

### 17.2 Response Writing Hot Path

**Embed-Eligible Asset Response:**
```go
func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    // Pre-computed header write - zero allocation
    w.Write(AssetMainCssHeader)
    
    // Method check - single if statement
    if r.Method != http.MethodHead {
        // Pre-computed content write - zero allocation
        w.Write(AssetMainCss)
    }
}
```

### 17.3 Conditional GET Hot Path

**ETag/Last-Modified Check:**
```go
// Pre-computed ETag and Last-Modified as constants
const AssetMainCssETag = "\"<imohash>\""
const AssetMainCssLastModified = "<RFC3339>"

func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    // Single conditional GET check
    if r.Header.Get("If-None-Match") == AssetMainCssETag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    if r.Header.Get("If-Modified-Since") == AssetMainCssLastModified {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    
    // Normal response
    w.Write(AssetMainCssHeader)
    if r.Method != http.MethodHead {
        w.Write(AssetMainCss)
    }
}
```

---

## 18. IMPLEMENTATION GUIDANCE

### 18.1 Generator Architecture

```
flashbuilder/
├── main.go           // CLI, entry point
├── asset.go          // Asset discovery, MIME detection
├── variant.go        // Variant generation (Brotli, AVIF, WebP)
├── budget.go         // Embed budget allocation
├── router.go         // Router generation, dispatch array
├── embed.go          // embed.go generation
├── main_gen.go       // main.go generation
├── tls.go            // TLS certificate generation
├── link.go           // Link creation
├── identifier.go     // Identifier generation, collision resolution
└── test.go           // Test generation
```

### 18.2 Generated Server Architecture

```
output/
├── assets/
│   └── embed.go      // All embed-eligible assets as constants
├── www/
│   └── <relpath>/    // Large assets as symlinks
├── main.go           // Router, dispatch array, per-length handlers
├── go.mod
├── go.sum
└── flash             // Compiled binary
```

### 18.3 Critical Implementation Notes

1. **All string operations MUST be eliminated from hot path**
2. **All map lookups MUST be eliminated from hot path**
3. **All interface dispatches MUST be eliminated from hot path**
4. **All header values MUST be pre-computed as `[]byte` constants**
5. **All embed-eligible content MUST be `//go:embed` directives**
6. **Router MUST use path length as direct array index**
7. **Path matching MUST use switch statements with compile-time known cases**
8. **Fallback MUST use recursive call to shorter-length handler**

---

## 19. SECURITY CONSIDERATIONS

### 19.1 Input Validation

- All paths MUST be sanitized before filesystem operations
- All paths MUST be relative to input directory
- No path traversal (`../`) allowed in generated routes

### 19.2 Output Validation

- All generated identifiers MUST be valid Go identifiers
- All generated file paths MUST be relative to output directory
- All symlinks MUST point to valid targets

### 19.3 Generated Server Security

- **Method validation:** Only `GET` and `HEAD` allowed
- **CSP headers:** Configurable per `--csp` flag
- **HSTS headers:** All TLS responses include HSTS
- **Alt-Svc headers:** All TLS responses include HTTP/3 advertisement
- **X-Frame-Options:** HTML responses include `DENY`
- **X-Content-Type-Options:** All responses include `nosniff`

---

## 20. COMPATIBILITY NOTES

### 20.1 Go Version Requirements

- Generator targets Go 1.26
- Generated server targets Go 1.26
- `//go:embed` requires Go 1.16+
- HTTP/3 (QUIC) requires Go 1.17+

### 20.2 CGO Requirements

- Generator requires CGO for Brotli, AVIF, WebP compression
- Generated server does NOT require CGO (pure Go)

### 20.3 Platform Requirements

- Generator targets Linux AMD64
- Generated server targets Linux AMD64
- Both use standard library where possible

---

## 21. FUTURE CONSIDERATIONS

### 21.1 Potential Optimizations

1. **Pre-computed HTTP/2 frame headers:** Reduce per-frame overhead
2. **Pre-computed QUIC frame headers:** Reduce per-packet overhead
3. **Custom HTTP parser:** Bypass `net/http` for ultra-low latency
4. **Custom TLS implementation:** Bypass `crypto/tls` for lower overhead
5. **io_uring integration:** Linux-only ultra-fast I/O

### 21.2 Potential Feature Additions

1. **Range request support:** For partial content delivery
2. **Compression negotiation:** Brotli/AVIF/WebP selection based on Accept header
3. **CDN integration:** Origin server for CDN pull-through
4. **Metrics endpoint:** Prometheus-compatible metrics
5. **Tracing integration:** OpenTelemetry tracing

---

## 22. SUMMARY

This specification defines `flashbuilder` as a Go code generator that produces `flash` - an ultra-high-performance static asset HTTP server. The generated server achieves:

- **Zero heap allocations** per request for embed-eligible assets
- **One array lookup** (dispatch by path length)
- **One switch statement** (per-length path matching)
- **One conditional GET check** (ETag/Last-Modified)
- **Pre-computed headers** as `[]byte` constants
- **Pre-computed content** via `//go:embed` directives

The generator follows a deterministic algorithm for asset discovery, deduplication, variant generation, budget allocation, and code generation. All decisions are made at generation time, ensuring the generated server has minimal runtime overhead.

**End of Specification.**
