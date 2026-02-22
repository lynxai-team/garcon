# FlashBuilder Specification

## Prompt

You are a **Senior Go 1.26 Code Generator Engineer** specializing in high-performance static-asset HTTP server generators. Your expertise encompasses Go code generation, CLI tool design, file system operations, HTTP/1.1/2/3 server architecture, idiomatic Go patterns, compile-time optimization, and KISS design principles.

Your task is to implement the complete `flashbuilder` tool following this specification exactly. This tool must generate a **single-binary Go 1.26 program** serving static assets with the **fastest possible request-processing hot-path**: that generated server, named `flash`, must achieve **at most** one array bounds check, one `switch`, and one conditional-GET `if` per request. Any computation performable at generation time **must** become a compile-time constant. Therefore, the `flashbuilder` tool must pre-compute all possible data and emit hard-coded code for the `flash` source code to minimize memory allocations and if/else branches.

## CLARIFICATION PROCEDURE  

If you encounter any of the following, **stop** and list each issue before proceeding:  
1. **Ambiguities** (e.g. unclear wording, missing defaults, unspecified behavior…)  
2. **Contradictions** (e.g. conflicting statements between sections…)  
3. **Infeasibilities** (e.g. required APIs, packages that do not exist…)

Prepare the decision to be easy for the human:  
- Analyze carefully every issue and merge together issues covering a similar topic.
- Order the issues from the most pertinent to the least interesting.
- Fusion the issues covering a similar subject.

Skip any issue already covered by a previous issue.

Provide a section for each issue, including:
- **Reference**: section number(s) where the issue appears.  
- **Best practices**: modern best practices about the domains implied, with the recommended approach
- **Actionable resolutions**: multiple concrete ways to resolve the issue, favoring overall simplification (e.g. minimizing the number of flags) according to these modern best practices.

Only after all open questions are answered should you continue with the specification generation.

---
---
---
---

## 1. Project Definition

### 1.1 Tool Identity

| Property | Value |
|----------|-------|
| Tool Name | `flashbuilder` |
| Generated Binary | `flash` |
| Target Language | Go 1.26 (released 2026) |
| Platform | Linux AMD64 |
| Generator Dependency Policy | CGO required for compression libraries |
| Generated Server Dependency Policy | Pure Go (no CGO) |

### 1.2 Performance Contract

The generated server must achieve:

| Constraint | Requirement |
|------------|-------------|
| Heap allocations | Zero per request for embed-eligible assets |
| Routing complexity | O(1): one dispatch array lookup, one switch statement |
| Conditional GET | Single `if` statement checking both headers |
| Pre-computation | All possible computations become compile-time constants |
| Embed mechanism | `//go:embed` directive for eligible assets |

### 1.3 Determinism Requirements

| Aspect | Behavior |
|--------|----------|
| Asset ordering | Fixed alphabetical order |
| Module name | Deterministic: `flash` |
| Identifier generation | Deterministic sanitization algorithm |
| Dependency versions | Latest compatible via `go mod tidy` |
| Non-determinism | Only TLS certificate randomness |

---

## 2. CLI Specification

### 2.1 Generator Command Syntax

```
flashbuilder <input> <output> [flags]
```

### 2.2 Generator Positional Arguments

| Name | Type | Constraints | Description |
|------|------|-------------|-------------|
| `input` | string | Must exist, must be directory, read-only | Path to asset tree |
| `output` | string | Must differ from input, created if missing | Destination for generated files |

### 2.3 Generator Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--embed-budget` | size | `0` | Cumulative embed budget in bytes. `0` means unlimited. `1` effectively disables embedding (only ≤1 byte assets). Negative values rejected with E025. Supports suffixes: K=1024, M=1024², G=1024³, T=1024⁴, KiB, MiB, GiB, TiB. |
| `--brotli` | int | `11` | Brotli compression level (0-11). Negative value disables Brotli. |
| `--avif` | int | `50` | AVIF encoding quality (0-100). Negative value disables AVIF. |
| `--webp` | int | `50` | WebP encoding quality (0-100). Negative value disables WebP. |
| `--csp` | string | `"default-src 'self'"` | Content-Security-Policy header for HTML assets. Empty string `""` disables CSP. Default is applied unless explicitly overridden. |
| `-v` | counter | `0` | Verbosity level: 0=WARN, 1=INFO, ≥2=DEBUG. Increment by adding flag multiple times. |
| `--dry-run` | bool | `false` | Simulate full pipeline without writing to output. Cache directory may be accessed. |
| `--tests` | bool | `false` | Run `go test ./... -race -vet=all` after generation. Failure exits with E079. |
| `--cache-max` | size | `5GiB` | Cache directory upper bound. Oldest-modified files evicted when exceeded. Supports same suffixes as embed-budget. |
| `--cache-dir` | path | XDG spec | Cache directory path. Must be on same filesystem as output. XDG resolution order: `$XDG_CACHE_HOME/flashbuilder` → `$HOME/.cache/flashbuilder` → `./.cache` |
| `--version` | bool | `false` | Print version and exit. |
| `--help` | bool | `false` | Show help and exit. |

### 2.4 Generator Environment Variables

| Flag | Environment Variable | Override Behavior |
|------|----------------------|-------------------|
| `--embed-budget` | `FLASHBUILDER_EMBED_BUDGET` | Overrides CLI flag |
| `--brotli` | `FLASHBUILDER_BROTLI` | Overrides CLI flag |
| `--avif` | `FLASHBUILDER_AVIF` | Overrides CLI flag |
| `--webp` | `FLASHBUILDER_WEBP` | Overrides CLI flag |
| `--csp` | `FLASHBUILDER_CSP` | Overrides CLI flag |
| `-v` | `FLASHBUILDER_LOG_LEVEL` | Overrides CLI flag |
| `--dry-run` | `FLASHBUILDER_DRY_RUN` | Overrides CLI flag |
| `--tests` | `FLASHBUILDER_TESTS` | Overrides CLI flag |
| `--cache-max` | `FLASHBUILDER_CACHE_MAX` | Overrides CLI flag |
| `--cache-dir` | `FLASHBUILDER_CACHE_DIR` | Overrides CLI flag |

All flag parsing must use `github.com/alecthomas/kong` exclusively. No manual `os.Getenv` calls are permitted.

### 2.5 Size Flag Parsing Implementation

```go
// Size suffix multipliers
const (
    K  = 1024
    M  = 1024 * 1024
    G  = 1024 * 1024 * 1024
    T  = 1024 * 1024 * 1024 * 1024
)

// ParseSize parses size strings like "4G", "100M", "5GiB"
func ParseSize(s string) (int64, error) {
    // Normalize: accept both "K"/"KiB" forms
    // Multiply by appropriate power of 1024
    // Return bytes as int64
}
```

### 2.6 Generated Server Command Syntax

```
flash [flags]
```

### 2.7 Generated Server Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--http` | string | `localhost:8080` | Plain-HTTP bind address. Value `none` disables. Supports HTTP/1.1 + h2c upgrade. |
| `--https` | string | `localhost:8443` | TLS bind address. Value `none` disables. Enables HTTP/2 via ALPN and HTTP/3 via QUIC on same port. |
| `--admin` | string | `localhost:8081` | Admin listener bind address. Value `none` disables. Exposes `/healthz`, `/readyz`, `/version`. |
| `-v` | counter | `0` | Verbosity: 0=WARN, 1=INFO, ≥2=DEBUG. |
| `--help` | bool | `false` | Show help and exit. |

### 2.8 Generated Server Environment Variables

| Flag | Environment Variable | Override Behavior |
|------|----------------------|-------------------|
| `--http` | `FLASH_HTTP` | Overrides CLI flag |
| `--https` | `FLASH_HTTPS` | Overrides CLI flag |
| `--admin` | `FLASH_ADMIN` | Overrides CLI flag |
| `-v` | `FLASH_LOG_LEVEL` | Overrides CLI flag |

### 2.9 Blocked Ports Validation

The generated server must reject binding to these ports (exit code 10, error E010):

```
1,7,9,11,13,15,17,19,20,21,22,23,25,37,42,43,53,69,77,79,87,95,101,102,103,
104,109,110,111,113,115,117,119,123,135,137,139,143,161,179,389,427,465,
512,513,514,515,526,530,531,532,540,548,554,556,563,587,601,636,989,990,
993,995,1719,1720,1723,2049,3659,4045,4190,5060,5061,6000,6566,6665,6666,
6667,6668,6669,6679,6697,10080
```

Implementation: Parse `--http` and `--https` values with `net.SplitHostPort`, extract port, check against blocked list.

---

## 3. Data Structures

### 3.1 Asset Struct

```go
type Asset struct {
    RelPath        string        // POSIX-style relative path from input (e.g., "css/main.css")
    AbsPath        string        // Absolute path to source file
    Size           int64         // File size in bytes
    MIME           string        // Detected MIME type
    ImoHash        uint64        // imohash of content (pre-computed ETag)
    IsDuplicate    bool          // True if content matches another asset
    CanonicalID    string        // Identifier of canonical asset (if duplicate)
    EmbedEligible  bool          // True if selected by embed-budget algorithm
    Variants       []Variant     // Kept variants (maximum one per type)
    HeaderLiteral  []byte        // Pre-computed header block
    Identifier     string        // Exported Go identifier (prefixed with "Asset")
    Filename       string        // Filename in assets/ directory
    FrequencyScore int           // Estimated request frequency (computed after MIME)
}
```

### 3.2 Variant Struct

```go
type Variant struct {
    Type          VariantType   // Compression type enum
    Size          int64         // Variant size in bytes
    HeaderLiteral []byte        // Pre-computed header bytes
    Identifier    string        // Go identifier for variant
    Extension     string        // File extension: "br", "avif", "webp"
    CachePath     string        // Absolute cache directory path
}
```

### 3.3 VariantType Enum

```go
type VariantType int

const (
    VariantBrotli VariantType = iota  // .br extension
    VariantAVIF                       // .avif extension
    VariantWebP                       // .webp extension
)
```

### 3.4 Router Path Maps

```go
var (
    canonicalPaths  map[string]string // canonical path → identifier
    duplicatePaths  map[string]string // duplicate path → canonical identifier
    shortcutPaths   map[string]string // shortcut path → canonical identifier
)
```

Map construction:
- Key: relative path string (without leading `/`)
- Value: Go identifier string (for canonical) or canonical identifier (for duplicate/shortcut)
- Collision rule: Before inserting, check all three maps; skip if key exists
- Shortcuts colliding with canonical or duplicate files are silently dropped

### 3.5 Server Struct

```go
type Server struct {
    logger   *slog.Logger
    dispatch [MaxLen+2]func(http.ResponseWriter, *http.Request)
    tlsCfg   *tls.Config
}
```

### 3.6 MaxLen Constant

```go
const MaxLen = <computed_at_generation>  // Maximum byte length of any relative path (no leading '/')
```

Computed from all canonical, shortcut, and duplicate paths.

---

## 4. Algorithms

### 4.1 Asset Discovery Algorithm

**Input**: `input` directory path  
**Output**: `[]Asset` slice  

```
ALGORITHM Discovery(input):
    assets = []
    filepath.Walk(input, func(path, info, err):
        if err != nil: return
        if info.IsDir(): return  // skip directories
        if info.Mode() & (os.ModeSocket | os.ModeDevice | os.ModeNamedPipe): return  // skip special files
        
        // Follow symlinks to files, skip symlinks to directories
        if info.Mode() & os.ModeSymlink:
            resolve target
            if target is directory: return
            path = resolved path
            info = target fileinfo
        
        relPath = filepath.Rel(input, path)  // POSIX-style
        absPath = absolute path
        size = info.Size()
        mime = DetectMIME(path)
        imohash = ComputeImoHash(path)
        freqScore = EstimateFrequencyScore(relPath, false)  // placeholder for isEmbed
        
        asset = Asset{RelPath: relPath, AbsPath: absPath, Size: size, MIME: mime, ImoHash: imohash, FrequencyScore: freqScore}
        assets.append(asset)
    )
    return assets SORTED BY relPath ALPHABETICALLY
```

### 4.2 MIME Detection Algorithm

**Input**: file path  
**Output**: MIME type string  

```go
func DetectMIME(path string) string {
    // Step 1: Extension-based lookup
    ext = filepath.Ext(path)  // lowercase, trim dots
    if ext != "" {
        mime = mime.TypeByExtension(ext)
        if mime != "" {
            return mime
        }
    }
    
    // Step 2: Content sniffing
    data = os.ReadFile(path)  // first 512 bytes
    mime = http.DetectContentType(data[:min(512, len(data))])
    if mime != "" {
        return mime
    }
    
    // Step 3: Fallback
    return "application/octet-stream"
}
```

HTML detection: MIME type `text/html` or variant containing `text/html` substring.

### 4.3 ImoHash Computation

```go
import "github.com/kalafut/imohash"

func ComputeImoHash(path string) uint64 {
    hasher := imohash.New()
    data, _ := os.ReadFile(path)
    hasher.Write(data)
    return hasher.Sum64()
}
```

### 4.4 Deduplication Algorithm

**Input**: `[]Asset`  
**Output**: `map[uint64][]Asset`, assets marked as duplicates  

```
ALGORITHM Deduplicate(assets):
    hashMap = make(map[uint64][]Asset)
    for each asset in assets:
        hashMap[asset.ImoHash].append(asset)
    
    for each group where len(group) > 1:
        // Verify content equality
        canonical = group[0]
        for each duplicate in group[1:]:
            if bytes.Equal(duplicate.content, canonical.content):
                duplicate.IsDuplicate = true
                duplicate.CanonicalID = canonical.Identifier
    
    return hashMap
```

Only canonical assets consume embed budget. Duplicates contribute zero budget cost.

### 4.5 Variant Keep Condition

```go
func ShouldKeepVariant(variantSize, originalSize int64) bool {
    // Keep variant if size reduction exceeds 3 KiB
    if variantSize < (originalSize - 3*1024) {
        return true
    }
    // Or reduction exceeds 10% of original size
    if variantSize < int64(float64(originalSize) * 0.9) {
        return true
    }
    return false
}
```

### 4.6 Variant Generation Rules

| Variant Type | Enabled When | Applies To | MIME Types |
|-------------|-------------|-----------|-----------|
| Brotli | `--brotli >= 0` | Assets ≥ 2 KiB | NOT `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/avif` (includes SVG, ICO) |
| AVIF | `--avif >= 0` | Any size | `image/png`, `image/jpeg` |
| WebP | `--webp >= 0` | Any size | `image/png`, `image/jpeg` |

Selection: If multiple variants satisfy keep condition, keep the **smallest**. Only one variant per asset.

### 4.7 Embed Budget Allocation Algorithm

```
ALGORITHM AllocateBudget(assets, budget):
    if budget == 0:
        // Unlimited: all non-duplicate assets are embed-eligible
        for each asset where NOT asset.IsDuplicate:
            asset.EmbedEligible = true
        return
    
    // Sort by size ascending
    sort(assets, by Size)
    
    cumulative = 0
    for each asset where NOT asset.IsDuplicate:
        if cumulative + asset.Size > budget:
            break
        asset.EmbedEligible = true
        cumulative += asset.Size
```

### 4.8 Identifier Generation Algorithm

```
ALGORITHM GenerateIdentifier(relPath):
    // Split path by "/" and "."
    segments = split(relPath, "/")
    segments = concat(segments, split(basename(relPath), "."))
    
    // Capitalize first letter of each segment
    for each segment in segments:
        segment = capitalizeFirstLetter(segment)
    
    // Trim non-alphanumeric characters
    for each segment:
        segment = trimNonAlphanumeric(segment)
    
    // Prefix with "Asset"
    identifier = "Asset" + join(segments, "")
    
    // Resolve collisions
    if exists(identifier):
        suffix = "_001"
        while exists(identifier + suffix):
            suffix = increment(suffix)  // _002, _003, ...
        identifier = identifier + suffix
    
    return identifier
```

### 4.9 Frequency Score Estimation

```go
func EstimateFrequencyScore(path string) int {
    score := 0

    // Root index
    if path == "" || path == "index.html" {
        score += 1000
    }
    
    // Root favicon
    if strings.Contains(path, "favicon.") {
        score += 900
    }
    
    // CSS files
    if strings.HasSuffix(path, ".css") {
        score += 800
    }
    
    // JavaScript files
    if strings.HasSuffix(path, ".js") {
        score += 600
    }
    
    // Other index files
    if strings.Contains(path, "index.html") {
        score += 500
    }
    
    // Logo images
    if strings.Contains(path, "logo.") {
        score += 400
    }
    
    // Long path penalty
    score -= 5 * len(path)
    
    // Deep directory penalty
    score -= 30 * strings.Count(path, "/")
    
    // Low-traffic extensions
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

### 4.10 MaxLen Computation

All the considered paths are relative, therefore none of them have a leading '/': no need to remove any eventual leading '/'.

```go
func ComputeMaxLen() int {
    maxLen := 0
    for path := range canonicalPaths {
        maxLen = max(maxLen, len(path))
    }
    // also check duplicate paths
    for path := range duplicatePaths {
        maxLen = max(maxLen, len(path))
    }
    // and shortcut paths
    for path := range shortcutPaths {
        maxLen = max(maxLen, len(path))
    }
    return maxLen
}
```

### 4.11 Link Creation Algorithm

**Case 1: Canonical embed-eligible with kept variant**
- Action: Create **hard link** in `assets/` as `Asset<Identifier>.<variant-ext>`
- Target: Variant file in cache directory (absolute path)
- No symbolic link for original

**Case 2: Canonical embed-eligible without variant**
- Action: Create **symbolic link** in `assets/` as `Asset<Identifier>.<ext>`
- Target: Original file in input (relative path computed via `filepath.Rel`)

**Case 3: Large asset (embed-ineligible)**
- Action: Create **symbolic link** in `www/<relpath>/<filename>`
- Target: Original file in input (relative path)
- Create intermediate directories as needed

**Case 4: Duplicate asset**
- Action: No file creation
- Router maps duplicate path to canonical identifier

**Relative Path Calculation**:
```go
// For symbolic links in assets/
relPath, err := filepath.Rel(output + "/assets/", input + "/" + asset.RelPath)

// For symbolic links in www/
relPath, err := filepath.Rel(output + "/www/" + asset.RelPath + "/", input + "/" + asset.RelPath + "/" + asset.Filename)
```

### 4.12 Cache Eviction Algorithm

```
ALGORITHM EvictCache(cacheDir, maxSize):
    files = list all files in cacheDir
    sort(files, by ModTime OLDEST FIRST)
    
    total = sum(file.Size for each file)
    for each file in files (oldest first):
        if total <= maxSize:
            break
        delete file
        total -= file.Size
```

### 4.13 Cache Filesystem Validation

```go
func ValidateCacheFilesystem(cacheDir, output string) error {
    cacheInfo, err := os.Stat(cacheDir)
    if err != nil {
        return err
    }
    outputInfo, err := os.Stat(output)
    if err != nil {
        return err
    }
    
    // Compare device IDs
    cacheDev := cacheInfo.Sys().(*syscall.Stat_t).Dev
    outputDev := outputInfo.Sys().(*syscall.Stat_t).Dev
    
    if cacheDev != outputDev {
        return fmt.Errorf("E087: cache and output on different filesystems")
    }
    return nil
}
```

### 4.14 Header Pre-computation

For each embed-eligible asset, compute header literal as `[]byte`:

```go
func BuildHeader(asset Asset, isHTTPS bool, csp string) []byte {
    var header strings.Builder
    
    // Content-Type
    header.WriteString("Content-Type: ")
    header.WriteString(asset.MIME)
    header.WriteString("\r\n")
    
    // Cache-Control
    if strings.HasPrefix(asset.RelPath, "index") || strings.HasSuffix(asset.RelPath, "/index.html") {
        header.WriteString("Cache-Control: public, max-age=31536000, immutable, must-revalidate\r\n")
    } else {
        header.WriteString("Cache-Control: public, max-age=31536000, immutable\r\n")
    }
    
    // Content-Length
    header.WriteString("Content-Length: ")
    header.WriteString(strconv.FormatInt(asset.Size, 10))
    header.WriteString("\r\n")
    
    // CSP for HTML
    if asset.MIME == "text/html" || strings.Contains(asset.MIME, "text/html") {
        if csp != "" {
            header.WriteString("Content-Security-Policy: ")
            header.WriteString(csp)
            header.WriteString("\r\n")
        }
    }
    
    // HTTPS-specific headers
    if isHTTPS {
        header.WriteString("Strict-Transport-Security: max-age=31536000\r\n")
        // Alt-Svc with HTTPS port extracted from bind address
        header.WriteString("Alt-Svc: h3=\":<port>\"; ma=2592000\r\n")
    }
    
    header.WriteString("\r\n")  // End of headers
    return []byte(header.String())
}
```

---

## 5. Router Implementation

### 5.1 Dispatch Array Structure

```go
var dispatch [MaxLen+2]func(http.ResponseWriter, *http.Request)
```

Each entry corresponds to a request path length L (0 to MaxLen+1 included) and the corresponding routes having length = (L-1). No nil entries permitted.

### 5.2 Dispatch Array Population Algorithm

```
ALGORITHM BuildDispatch(assets):
    MaxLen = ComputeMaxLen(assets)
    dispatch = make([]func(http.ResponseWriter, *http.Request), MaxLen+2)
    
    // Root index handling
    if hasRootIndex():
        dispatch[0] = serveRootIndex()
        dispatch[1] = serveRootIndex()
    else:
        dispatch[0] = http.NotFound
        dispatch[1] = http.NotFound
    
    // For each length L from 2 to MaxLen+1:
    for L = 2 to MaxLen+1:
        routes = collect all paths of length L-1 (canonical, duplicate, shortcut)
        if len(routes) > 0:
            sortRoutesByFrequencyScore(routes)  // descending
            dispatch[L] = generatePerLengthHandler(L, routes)
        else:
            // Find nearest valid dispatch
            K = find nearest valid dispatch index < L
            if K >= 0:
                dispatch[L] = dispatch[K]
            else:
                dispatch[L] = http.NotFound
    
    return dispatch
```

### 5.3 Per-Length Handler Template

```go
func handleLen<L>(w http.ResponseWriter, r *http.Request) {
    const L = <L>
    // Safety: dispatch array ensures len(r.URL.Path) >= L when this is called
    pathNoSlash = r.URL.Path[1:]    // Remove leading '/'
    truncated = pathNoSlash[:L-1]   // truncated length is L-1

    switch truncated {
    case "<path1>":  // Highest frequency score
        assets.ServeAssetXXX(w, r)
    case "<path2>":  // Second highest
        assets.ServeAssetYYY(w, r)
    // ... all paths of length L-1 ordered by descending score
    default:
        dispatch[L-1](w, r) // Fallback to nearest prefix
    }
}
```

### 5.4 Top-Level Router

```go
func (s *Server) router(w http.ResponseWriter, r *http.Request) {
    idx := min(len(r.URL.Path), len(s.dispatch)-1)
    s.dispatch[idx](w, r)
}
```

### 5.5 Duplicate and Shortcut Path Handling

For each entry in `duplicatePaths` and `shortcutPaths`:

```go
// In per-length handler switch statement:

case "<duplicate_path>":
    // Map to canonical handler
    assets.Serve<CanonicalIdentifier>(w, r)

case "<shortcut_path>":
    // Map to canonical handler
    assets.Serve<CanonicalIdentifier>(w, r)
```

The duplicate/shortcut path string is the switch key; the canonical identifier determines the handler.

### 5.6 Large Asset Handler Template

```go
func serveAsset<Identifier>(w http.ResponseWriter, r *http.Request) {
    // No pre-computed headers; http.ServeFile handles everything
    http.ServeFile(w, r, "www/<relpath>/<filename>")
}
```

---

## 6. Code Templates

### 6.1 embed.go Template

```go
// Package assets contains all embed-eligible assets.
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

var AssetAboutIndexHtmlHeader []byte = []byte("...")
var AssetMainCssHeader []byte = []byte("...")
// ... all header literals ...

func ServeAssetAboutIndexHtml(w http.ResponseWriter, r *http.Request) {
    // check conditional GET (support ETag only)
    if r.Header.Get("If-None-Match") == etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    w.Write(AssetAboutIndexHtmlHeader)
    if r.Method != "" && r.Method[0] == 'H' {
        return  // HEAD request
    }
    w.Write(AssetAboutIndexHtml)
}

func ServeAssetMainCss(w http.ResponseWriter, r *http.Request) {
    // check conditional GET (support ETag only)
    if r.Header.Get("If-None-Match") == etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    w.Write(AssetMainCssHeader)
    if r.Method != "" && r.Method[0] == 'H' {
        return  // HEAD request
    }
    w.Write(AssetMainCss)
}

// ... all handler functions ...
```

Note: ETag is pre-computed as `ImoHash` (hex string). Last-Modified is not supported because modern major browsers prefer ETag.

### 6.2 TLS Configuration (ECDSA P256)

```go
import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/hex"
    "math/big"
    "time"
)

func generateTLSConfig() *tls.Config {
    key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        panic(err)
    }
    
    // Random 64-bit serial
    serial, _ := rand.Int(rand.Reader, big.NewInt(1<<64-1))
    
    cert := &x509.Certificate{
        SerialNumber: serial,
        Subject:      pkix.Name{CommonName: "flash"},
        NotBefore:    time.Now(),
        NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
    }
    
    certBytes := x509.CreateCertificate(rand.Reader, cert, cert, &key.Public(), key)
    
    return &tls.Config{
        MinVersion:  tls.VersionTLS13,
        NextProtos:  []string{"h3", "h2", "http/1.1"},
        Certificates: []tls.Certificate{
            {Certificate: certBytes, PrivateKey: key},
        },
    }
}
```

### 6.4 HTTP/2 h2c Configuration

```go
import "golang.org/x/net/http2"

func configureHTTP2(server *http.Server) {
    h2srv := &http2.Server{}
    h2srv.ConfigureServer(server, nil)  // Enable h2c upgrade
}
```

### 6.5 HTTP/3 QUIC Listener

```go
import "github.com/quic-go/quic-go/http3"

func startHTTP3(httpsAddr string, tlsCfg *tls.Config) *http3.Server {
    return &http3.Server{
        Addr:      httpsAddr,
        TLSConfig: tlsCfg,
    }
}
```

### 6.6 Graceful Shutdown Handler

```go
func setupGracefulShutdown(httpSrv *http.Server, httpsSrv *http.Server, http3Srv *http3.Server, adminSrv *http.Server, logger *slog.Logger) {
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
                wg.Add(4)
                
                go func() { defer wg.Done(); _ = httpSrv.Shutdown(ctx) }()
                go func() { defer wg.Done(); _ = httpsSrv.Shutdown(ctx) }()
                go func() { defer wg.Done(); _ = http3Srv.Close() }()
                go func() { defer wg.Done(); _ = adminSrv.Shutdown(ctx) }()
                
                wg.Wait()
                logger.Info("shutdown complete")
                os.Exit(0)
            }
        }
    }()
}
```

### 6.7 Health Check Handlers

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
    w.Header().Set("Content-Type", "application/json")
    w.Write(versionResponse)  // Pre-computed JSON constant
}
```

### 6.8 Admin Handler

```go
func adminHandler(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/healthz":
        healthzHandler(w, r)
    case "/readyz":
        readyzHandler(w, r)
    case "/version":
        versionHandler(w, r)
    case "/debug/pprof/":
        pprof.Index(w, r)
    default:
        http.NotFound(w, r)
    }
}
```

---

## 7. Dependencies

### 7.1 Generator Dependencies (CGO Required)

| Package | Import Path | CGO Requirement | System Libraries |
|---------|------------|-----------------|------------------|
| Kong | `github.com/alecthomas/kong` | None | None |
| ImoHash | `github.com/kalafut/imohash` | None | None |
| Brotli | `github.com/google/brotli/go/cbrotli` | CGO | `libbrotli-dev` |
| AVIF | `github.com/vegidio/avif-go` | CGO | `libavif-dev` |
| WebP | `github.com/kolesa-team/go-webp/encoder` | CGO | `libwebp-dev` |

Build requirement: `CGO_ENABLED=1` with system libraries installed.

### 7.2 Generated Server Dependencies (Pure Go)

| Package | Import Path | Purpose |
|---------|------------|---------|
| Kong | `github.com/alecthomas/kong` | CLI parsing |
| HTTP/2 | `golang.org/x/net/http/http2` | h2c upgrade |
| HTTP/3 | `github.com/quic-go/quic-go/http3` | QUIC listener |
| Assets | `flash/assets` | Local package |

---

## 8. Error Handling

### 8.1 Error Codes

| Code | Description | Exit Status |
|------|-------------|-------------|
| E001 | I/O error reading asset | 2 |
| E010 | Blocked port violation | 10 |
| E025 | Invalid compression/budget level | 2 |
| E030 | Dry-run permission validation failure | 2 |
| E087 | Link creation failure or cross-filesystem hard-link | 87 |
| E099 | Generic internal error | 2 |
| E079 | Test suite failure | 3 |

### 8.2 Error Message Format

All error messages are prefixed with error code:

```
E087: Failed to create symbolic link: permission denied
```

Logged via `slog` at ERROR level.

---

## 9. Security Headers

### 9.1 Header Generation Rules

| Header | Condition | Value |
|--------|-----------|-------|
| Content-Type | All responses | Detected MIME type |
| Cache-Control | Index files | `public, max-age=31536000, immutable, must-revalidate` |
| Cache-Control | Non-index files | `public, max-age=31536000, immutable` |
| Content-Length | All responses | Asset size |
| Content-Security-Policy | HTML only | Value from meta tag or `--csp` flag |
| Strict-Transport-Security | HTTPS only | `max-age=31536000` |
| Alt-Svc | HTTPS only | `h3=":<port>"; ma=2592000` |

### 9.2 CSP Extraction from HTML

```go
func ExtractCSP(html string) string {
    lowerHTML := strings.ToLower(html)
    idx := strings.Index(lowerHTML, `meta http-equiv="content-security-policy"`)
    if idx == -1 {
        return ""
    }
    contentIdx := strings.Index(lowerHTML[idx:], `content="`)
    if contentIdx == -1 {
        return ""
    }
    start := idx + contentIdx + 9
    end := strings.Index(lowerHTML[start:], `"`)
    if end == -1 {
        return ""
    }
    return html[start : start+end]
}
```

### 9.3 CSP Priority

1. If HTML contains `<meta http-equiv="Content-Security-Policy" content="...">`, use extracted value
2. Else if `--csp=""` is explicitly set, no CSP header
3. Else use `--csp` default value (`"default-src 'self'"`)

---

## 10. Output Structure

### 10.1 Directory Layout

```
output/
├── assets/
│   ├── embed.go                 // All embed-eligible assets
│   ├── Asset<Identifier>.<ext>  // Links to originals (symlinks) or variants (hard links)
│   └── ...
├── www/
│   └── <relpath>/<filename>     // Large assets (symbolic links)
├── main.go                      // Router and handlers
├── go.mod
├── go.sum
└── flash                        // Compiled binary
```

### 10.2 Symbolic vs Hard Links

| Link Type | Location | Target | Path Type |
|-----------|----------|--------|-----------|
| Hard link | `assets/` | Variant in cache | Absolute path |
| Symbolic link | `assets/` | Original in input | Relative path |
| Symbolic link | `www/` | Original in input | Relative path |

---

## 11. Implementation Workflow

### 11.1 Step Sequence

1. **Parse CLI**: Use `kong` for flag parsing, validate environment overrides
2. **Validate Arguments**: Input != output, cache filesystem validation
3. **Create Directories**: Output, cache (if needed)
4. **Discover Assets**: Walk input, record metadata, compute `FrequencyScore`
5. **Detect MIME**: Extension-based, content sniffing, fallback
6. **Compute ImoHash**: For each asset
7. **Deduplicate**: Build `hashMap`, mark duplicates
8. **Generate Identifiers**: Sanitize paths, resolve collisions
9. **Generate Variants**: Brotli, AVIF, WebP (creates cache directory)
10. **Select Variants**: Keep smallest satisfying condition
11. **Allocate Embed Budget**: Greedy smallest-first selection
12. **Update Frequency Scores**: With `isEmbed` status
13. **Pre-compute Headers**: Build `[]byte` literals
14. **Create Links**: Hard links for variants, symbolic links for originals
15. **Build Path Maps**: `canonicalPaths`, `shortcutPaths`, `duplicatePaths`
16. **Generate embed.go**: `//go:embed` directives, header literals, handlers
17. **Generate main.go**: Router, dispatch array, per-length handlers
18. **Initialize Module**: `go mod init flash`, `go mod tidy`
19. **Build Binary**: `go build -trimpath -ldflags="-s -w" -o flash ./...`
20. **Run Tests**: `go test ./... -race -vet=all`
21. **Exit**: Return appropriate exit code

### 11.2 Determinism Guarantees

| Step | Deterministic Aspect |
|------|---------------------|
| Asset ordering | Fixed alphabetical by `RelPath` |
| Identifier generation | Deterministic sanitization |
| Module name | Fixed: `flash` |
| Variant selection | Deterministic keep condition |
| TLS certificate | Random serial (only source of non-determinism) |

---

## 12. Testing Requirements

### 12.1 Test Types

| Type | Description | Tools |
|------|-------------|-------|
| Unit | Identifier sanitization, MIME detection, header rendering, budget logic | `go test` |
| Integration | Server startup, all protocols, response headers, variant selection | `go test` with test servers |
| Fuzz | `FuzzIdentifierCollision` (random paths), `FuzzRouterPath` (random request paths) | `go test -fuzz` |
| Benchmark | Per-request latency, allocations, throughput | `go test -bench` with `-benchmem` |
| Error-path | All error codes (E001, E010, E087, E079) | Mock failures |

### 12.2 Performance Benchmarks

| Metric | Target |
|--------|--------|
| Embed-eligible latency | ≤100ns |
| Embed-eligible allocations | 0 bytes |
| Cached variant latency | ≤500ns |
| Large file latency | ≤1μs |

---

## 13. Glossary

| Term | Definition |
|------|------------|
| **Input** | First positional argument; directory containing asset tree (read-only) |
| **Output** | Second positional argument; destination for generated files |
| **Asset** | Regular file or symlink to regular file under input |
| **Variant** | Transformed representation: Brotli-compressed (.br), AVIF-encoded (.avif), WebP-encoded (.webp) |
| **Embed-eligible** | Asset selected by embed-budget algorithm, compiled into binary via `//go:embed` |
| **Large asset** | Asset not embed-eligible; served via `http.ServeFile` from `www/` |
| **Canonical path** | POSIX-style relative path from input (e.g., `css/main.css`) |
| **Shortcut** | Canonical path without extension (non-index) or without `/index.html` (index files) |
| **Router** | Compile-time generated dispatch array plus per-length switch statements |
| **Cache directory** | XDG-compliant directory holding transformed variants |
| **MaxLen** | Compile-time constant: byte length of longest path without leading `/` |
| **Hot path** | Request processing with one bounds check, one switch, one conditional GET if |
| **Frequency score** | Estimated request traffic frequency used to order switch cases |
