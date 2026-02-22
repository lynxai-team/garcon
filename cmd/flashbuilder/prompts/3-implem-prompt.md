# IMPLEMENTATION PROMPT

## IMPLEMENTATION PROMPT FOR FLASHBUILDER

You are a **Senior Go 1.26 Code Generator Engineer** implementing `flashbuilder` - a static asset HTTP server generator tool. Your task is to implement the complete `flashbuilder` tool following this specification exactly.

---

## 1. PRIMARY GOAL

You must implement `flashbuilder`. This tool must generate a **single-binary Go 1.26 program** serving static assets with the **fastest possible request-processing hot-path**: that generated server, named `flash`,  must achieve **at most** one array bounds check, one `switch`, and one conditional-GET `if` per request. Any computation performable at generation time **must** become a compile-time constant. Therefore, the `flashbuilder` tool must pre-compute all possible data and emit hard-coded code for the `flash` source code to minimize memory allocations and if/else branches.

---

## 2. TARGET PLATFORM

| Item | Specification |
|------|---------------|
| Generator Target | Go 1.26, Linux AMD64, CGO required |
| Generated Server Target | Go 1.26, Linux AMD64, pure Go (no CGO) |
| Generator Dependencies | Go standard library, `github.com/alecthomas/kong`, `github.com/kalafut/imohash`, `github.com/google/brotli/go/cbrotli`, `github.com/vegidio/avif-go`, `github.com/kolesa-team/go-webp/encoder` |
| Generated Server Dependencies | Go standard library, `github.com/alecthomas/kong`, `golang.org/x/net/http2`, `github.com/quic-go/quic-go/http3` |

---

## 3. CLI SPECIFICATION

### 3.1 Generator CLI (`flashbuilder`)

**Usage:** `flashbuilder <input> <output> [flags]`

**Positional Arguments:**

| Argument | Type | Description |
|----------|------|-------------|
| `input` | string | Path to asset tree (read-only, never modified) |
| `output` | string | Destination for generated files. Must not equal input. Created if missing. |

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
| `--cache-dir` | path | `kong.Path` | `$FLASHBUILDER_CACHE_DIR` | Cache directory. Must be same filesystem as output. Default follows XDG: `$XDG_CACHE_HOME/flashbuilder` or `$HOME/.cache/flashbuilder`. |
| `--version` | bool | `kong.Bool` | `false` | Print version and exit. |
| `--help` | bool | `kong.Bool` | `false` | Show usage. |

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

**Size Parsing:** Parse integer with optional suffix. `K`=1024, `M`=1024², `G`=1024³, `T`=1024⁴. Example: `--embed-budget=4G` = 4×1024³ = 4,294,967,296 bytes.

### 3.2 Generated-Server CLI (`flash`)

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

**All flag parsing must use `kong` exclusively. No `os.Getenv` calls permitted.**

---

## 4. DATA STRUCTURES

### 4.1 Asset Struct

```go
type Asset struct {
    RelPath       string        // POSIX-style path relative to input: "css/main.css"
    AbsPath       string        // Absolute input path (for symlink creation)
    Size          int64         // Raw file size in bytes
    ModTime       time.Time     // Modification time (UTC)
    MIME          string        // Detected MIME type
    ImoHash       uint64        // imohash of raw content
    IsDuplicate   bool          // True if another asset shares identical content
    CanonicalID   string        // Identifier of canonical asset (if duplicate)
    EmbedEligible bool          // True if selected by embed-budget
    Variants      []Variant     // Kept variants (max one per type)
    HeaderLiteral []byte        // Pre-computed header block (embed-eligible only)
    Identifier    string        // Exported Go identifier (prefixed with "Asset")
    Filename      string        // Filename in assets/: "AssetMainCss.css"
}
```

### 4.2 Variant Struct

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

### 4.3 VariantType Constants

```go
type VariantType int

const (
    VariantBrotli VariantType = iota
    VariantAVIF
    VariantWebP
)
```

### 4.4 Router Path Maps

```go
var (
    canonicalPaths  map[string]string // canonical path → identifier
    shortcutPaths   map[string]string // shortcut path → identifier
    duplicatePaths  map[string]string // duplicate path → canonical identifier
)
```

---

## 5. ALGORITHMS

### 5.1 Asset Discovery Algorithm

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

### 5.2 MIME Detection Algorithm

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

**HTML detection:** `mime` starts with `"text/html"` → is HTML.

### 5.3 Imohash Algorithm

```go
import "github.com/kalafut/imohash"

func computeImohash(file io.Reader) uint64 {
    hasher := imohash.New()
    hasher.Write(readAllBytes(file))
    return hasher.Sum64()
}
```

### 5.4 Deduplication Algorithm (BEFORE Budget Calculation)

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

**CRITICAL:** Deduplication occurs BEFORE budget calculation.

### 5.5 Variant Keep Condition

```
FUNCTION should_keep_variant(variant_size, original_size):
    // Keep if EITHER condition satisfied:
    // (variant_size < original_size - 3 KiB) OR (variant_size < 0.9 * original_size)
    
    if variant_size < (original_size - 3*1024):
        return true
    if variant_size < (0.9 * original_size):
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

### 5.6 Embed Budget Allocation Algorithm

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

### 5.7 Identifier Generation Algorithm

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

### 5.8 Frequency Estimation Algorithm

```go
func estimateFrequencyScore(path string, isEmbedEligible bool, filename string) int {
    if path == "" || path == "index.html":
        return 1000
    if strings.HasSuffix(path, "favicon."):
        return 900
    if strings.HasSuffix(path, ".css"):
        return 800
    if strings.HasSuffix(path, ".js"):
        return 600
    if strings.Contains(filename, "Index"):
        return 500
    if strings.Contains(path, "logo."):
        return 400
    
    score := 0
    if isEmbedEligible {
        score += 200
    }
    score -= 5 * len(filename)
    score -= 20 * strings.Count(path, "/")
    
    lowTraffic := []string{"Map", "Zip", "Pdf", "Doc", "Xls", "Tar"}
    for _, ext := range lowTraffic {
        if strings.Contains(path, ext) {
            score -= 100
            break
        }
    }
    return score
}
```

### 5.9 Dispatch Array Population Algorithm

```
FUNCTION build_dispatch_array(all_paths):
    MaxLen = compute_maxlen(all_paths)
    dispatch = make([]func(http.ResponseWriter, *http.Request), MaxLen+1)
    
    for L = 0 to MaxLen:
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
    
    if dispatch[0] == nil OR dispatch[0] == http.NotFound:
        if has_root_index():
            dispatch[0] = serve_root_index()
        else:
            dispatch[0] = http.NotFound
    
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

### 5.10 Router Fallback Algorithm

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

### 5.11 Link Creation Algorithm

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

## 6. CODE TEMPLATES

### 6.1 embed.go Template

```go
// Package assets contains all embed-eligible assets.
// Generated by flashbuilder - DO NOT EDIT
package assets

import (
    _ "embed"
    "net/http"
)

//go:embed AssetAboutIndexHtml.html
var AssetAboutIndexHtml []byte

//go:embed AssetMainCss.css
var AssetMainCss []byte

// ... all embed-eligible assets ...

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

### 6.2 Header Literal Format

For each embed-eligible asset:
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

### 6.3 main.go Template

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

func handleLen0(w http.ResponseWriter, r *http.Request) {
    // Generated switch for length 0
}

// ... up to handleLen<MaxLen> ...

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

### 6.4 Per-Length Handler Template

```go
func handleLen<LENGTH>(w http.ResponseWriter, r *http.Request) {
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

### 6.5 Health Endpoints

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

### 6.6 Graceful Shutdown

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

## 7. ERROR HANDLING

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

## 8. WORKFLOW STEPS

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

## 9. DEPENDENCIES

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

## 10. BLOCKED PORTS

The following ports are blocked (Fetch standard, https://fetch.spec.whatwg.org/#port-blocking):

```
1, 7, 9, 11, 13, 15, 17, 19, 20, 21, 22, 23, 25, 37, 42, 43, 53, 69, 77, 79, 87, 95, 101, 102, 103, 104, 109, 110, 111, 113, 115, 117, 119, 123, 135, 137, 139, 143, 161, 179, 389, 427, 465, 512, 513, 514, 515, 526, 530, 531, 532, 540, 548, 554, 556, 563, 587, 601, 636, 989, 990, 993, 995, 1719, 1720, 1723, 2049, 3659, 4045, 4190, 5060, 5061, 6000, 6566, 6665, 6666, 6667, 6668, 6669, 6679, 6697, 10080
```

Generated server must exit with E010 if any bind address uses a blocked port.

---

## 11. SECURITY HEADERS

| Header | When | Value |
|--------|------|-------|
| CSP | HTML assets only | `--csp` flag: empty → disabled, unset → `"default-src 'self'"`, custom → use value |
| HSTS | Every TLS response | `max-age=31536000` |
| Alt-Svc | Every TLS response | `h3=":<port>"; ma=2592000` |
| X-Content-Type-Options | Known MIME | `nosniff` |
| X-Frame-Options | HTML assets | `DENY` |
| Method validation | All requests | Only `GET` and `HEAD` allowed; `405 Method Not Allowed` for others |

---

## 12. TESTING REQUIREMENTS

- **Unit Tests:** Identifier sanitization, MIME detection, header rendering, embed-budget logic, directory creation, link path calculation, frequency estimation.
- **Integration Tests:** Spin up generated server on random ports (`net.Listen(":0")`), exercise all protocol listeners, verify response headers, verify variant selection, verify `Alt-Svc`/`HSTS`.
- **Fuzz Tests:** `FuzzIdentifierCollision` (random path strings), `FuzzRouterPath` (random request paths).
- **Benchmarks:** Per-request latency, allocations, throughput. Verify zero heap allocations for hot path using `go test -bench=. -benchmem`.

---

## 13. OUTPUT STRUCTURE

```
output/
├── assets/
│   ├── embed.go                  // single file with all embed-eligible assets
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

## 14. DETERMINISM

- Fixed alphabetical ordering of assets
- Deterministic module name: `flash`
- `go mod init flash` and `go mod tidy`
- TLS certificate randomness is the only source of nondeterminism

---

## 15. KISS COMPLIANCE

Both generator and generated server must remain easy to read, reason about, and maintain. When a trade-off is required, **runtime performance always takes precedence** over brevity. The generated server must achieve the fastest possible request-processing hot-path with at most one array bounds check, one switch, and one conditional-GET if per request.

---

## 16. PERFORMANCE CONSTRAINTS

**Hot-path requirements:**
- Zero heap allocations per request for embed-eligible assets
- One array lookup (`dispatch[len(path)]`)
- One switch (per-length handler)
- One conditional-GET if (`If-None-Match`/`If-Modified-Since`)
- All headers pre-computed as compile-time constants
- All embed-eligible assets as `[]byte` literals via `//go:embed`

---

## IMPLEMENTATION INSTRUCTIONS

Implement the complete `flashbuilder` tool following this specification exactly. The implementation must:

1. Parse all CLI flags using `kong` exclusively
2. Discover assets recursively following the exact algorithm
3. Detect MIME types in the exact order specified
4. Deduplicate BEFORE budget calculation
5. Generate variants via CGO libraries
6. Apply the exact keep condition logic
7. Allocate budget using greedy smallest-first
8. Pre-compute all headers as byte literals
9. Generate identifiers with collision resolution
10. Create links following the four cases
11. Build router with frequency-ordered dispatch
12. Generate `embed.go` and `main.go` exactly as specified
13. Run `go mod init flash` and `go mod tidy`
14. Run `go build` and `go test`
15. Handle all errors with proper exit codes

**END OF IMPLEMENTATION PROMPT**
