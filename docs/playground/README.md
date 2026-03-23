# Shoutrrr Playground

A browser-based interactive tool for configuring, generating, and testing Shoutrrr notification URLs. Built with Go WASM, all processing runs client-side — no data leaves the user's device.

## Architecture

```bash
docs/playground/
├── index.md              # MkDocs page content
├── assets/
│   ├── shoutrrr-playground.js   # Frontend: WASM loader, UI logic
│   ├── shoutrrr-playground.css  # Styling
│   ├── shoutrrr.wasm            # Compiled WASM binary (build output)
│   └── wasm_exec.js             # Go WASM runtime (build output)
└── wasm/
    ├── doc.go            # Package documentation
    ├── types.go          # JSON-serializable type definitions
    ├── schema.go         # Service listing and config schema generation
    ├── parser.go         # URL parsing and validation
    ├── generator.go      # URL generation from config values
    ├── helpers.go        # Utility functions
    ├── main.go           # WASM entry point, JS function registration
    ├── fetch.go          # Send implementation using Shoutrrr
    ├── wasm_suite_test.go     # Ginkgo test suite
    ├── helpers_test.go        # Helper function tests
    ├── schema_test.go         # Schema generation tests
    ├── parser_test.go         # URL parsing tests
    ├── generator_test.go      # URL generation tests
    ├── main_test.go           # JS binding tests (WASM-only)
    ├── fetch_test.go          # Send function tests (WASM-only)
    └── wasm_test.go           # Integration-style tests
```

## How It Works

1. **WASM Module**: The Go code compiles to WASM and exposes functions to JavaScript via `syscall/js`:
   - `shoutrrrGetServices()` — lists all registered service schemes
   - `shoutrrrGetConfigSchema(service)` — returns field metadata for a service
   - `shoutrrrParseURL(url)` — parses a Shoutrrr URL into config values
   - `shoutrrrGenerateURL(service, config)` — builds a Shoutrrr URL from config
   - `shoutrrrValidateURL(url)` — validates a Shoutrrr URL
   - `shoutrrrSend(url, message)` — sends a notification (returns Promise)

2. **Automatic Parity**: The WASM module calls the same public API as the Shoutrrr CLI:
   - `router.ListServices()` — reads directly from `serviceMap`
   - `router.NewService()` — creates service instances
   - `format.GetServiceConfig()` — extracts service config
   - `format.GetConfigFormat()` — introspects struct tags via reflection

   New services, changed fields, and updated defaults propagate automatically on rebuild.

3. **Send Functionality**: Uses Shoutrrr's `Send()` function directly. The frontend patches `window.fetch` to strip the `User-Agent` header, which some CORS preflight responses don't allow.

## Build

```bash
make wasm
```

This compiles the Go source to WASM and copies `wasm_exec.js` from the Go toolchain:

```bash
GOOS=js GOARCH=wasm go build \
    -trimpath \
    -o docs/playground/assets/shoutrrr.wasm \
    -ldflags="-s -w" \
    ./docs/playground/wasm/
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" docs/playground/assets/
```

### Build Flags

| Flag            | Purpose                                                                    |
|-----------------|----------------------------------------------------------------------------|
| `-trimpath`     | Removes file system paths from the binary for security and reproducibility |
| `-ldflags="-s"` | Strips the symbol table                                                    |
| `-ldflags="-w"` | Strips DWARF debug info                                                    |

## Test

```bash
go test ./docs/playground/wasm/
```

Tests use Ginkgo/Gomega and cover all pure logic functions. WASM-only tests (`main_test.go`, `fetch_test.go`) are skipped in non-WASM builds.

## Lint

```bash
golangci-lint run --config build/golangci-lint/golangci.yaml ./docs/playground/wasm/
```

## MkDocs Integration

The playground is served as a MkDocs page with:

- Navigation entry: `Playground: playground/index.md` in `build/mkdocs/mkdocs.yaml`
- WASM source excluded from docs processing: `exclude_docs: try-it/wasm/`
- Conditional asset loading in `docs/overrides/main.html`

## Adding Service Support

The playground supports all Shoutrrr services automatically. Special handling exists for services with unexported `*url.URL` fields (e.g., generic):

- A synthetic "WebhookURL" field is added to the config schema
- The `SetURL()` method is called via reflection to initialize the config
- The webhook URL is extracted via the `WebhookURL()` method when parsing

No service-specific code changes are needed — the playground uses Shoutrrr's public API.
