# goargv (Go)

A lightweight, zero-dependency, robust CLI argument parser for Go.

Designed as a modern alternative to Go's standard `flag` package, `goargv` lets you bind options and positional arguments directly to target pointers while providing short flag clustering, boolean negation, automatic default value detection, environment variable fallbacks, and multi-flag logical rules.

---

## Why use this over Go's standard `flag`?

Go's built-in `flag` package is reliable but lacks features expected in modern CLI tools. `goargv` bridges this gap with zero third-party dependencies:

- **Zero external dependencies**: Built purely on the Go standard library (`errors`, `fmt`, `io`, `os`, `path/filepath`, `strconv`, `strings`).
- **Flexible flag definition**: Pass `"v"` and `"verbose"` or `"-v"` and `"--verbose"`—prefixes are automatically normalized.
- **Built-in boolean negation**: Supports `--flag`, `--no-flag`, and explicit inline values (`--flag=false`, `--flag=true`, `1`, `0`, `yes`, `no`, `on`, `off`). Formatted automatically as `--[no-]flag` in help screens.
- **Automatic default values**: Displays existing defaults (`[default: ...]`) in help screens for strings, integers, floats, characters, and `true` booleans.
- **Global environment prefix**: Define `SetEnvPrefix("APP")` to automatically map long flags like `--port` to `APP_PORT` without repetitive manual bindings.
- **Testable via `io.Writer`**: Redirect help, version, and usage text to any `io.Writer` (e.g. `bytes.Buffer`) with `SetOutput` for effortless unit testing.
- **Short flag clustering**: Bundles boolean and counter flags seamlessly (e.g., `-xvf archive.tar.gz` or `-vvv`).
- **Documented choices**: Constrain inputs to allowed choices and attach per-choice explanations (`SetChoiceHelp`).
- **Logical relationships**: Enforce mutual exclusion across groups (`AddExclusive`) and define prerequisite flag dependencies (`AddDependency`) with upfront validation.
- **First-class positional & rest arguments**: Easily declare mandatory/optional positional arguments and trailing slices (`BindRest`).
- **Idiomatic error handling**: Inspect failures cleanly using standard `errors.Is(err, goargv.ErrMutualExclusion)`.
- **Nil-safe & reusable**: Guarded against nil pointer targets and clean across multiple `Parse()` calls.

---

## Installation

```bash
go get github.com/Mukumbaa/goargv
```

---

## Quick Example

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Mukumbaa/goargv"
)

func main() {
	p := goargv.New("datax")

	// 1. Declare target variables with default values
	var (
		dryRun     bool
		compress   bool = true // Shows [default: true] in help
		verbosity  int
		mode       byte    = 'r'
		port       int     = 8080 // Shows [default: 8080] in help
		format     string  = "json"
		token      string
		encrypt    bool
		plain      bool
		key        string
		inputFile  string
		extraFiles []string
	)

	// Optional: configure version banner (-V / --version)
	p.SetVersion("1.2.0", "V", "Build 2026.1 - Open Source Tool")

	// Optional: automatically read unset flags from DATAX_* environment variables
	p.SetEnvPrefix("DATAX")

	// 2. Bind flags (leading hyphens are optional)
	p.BindBool("d", "dry-run", &dryRun, "Simulate execution without modifying files")
	p.BindBool("c", "compress", &compress, "Enable payload compression")
	p.BindCount("v", "verbose", &verbosity, "Increase verbosity level (-v, -vv, -vvv)")
	p.BindChar("m", "mode", &mode, false, "r,w,a", "File access mode")
	p.BindInt("p", "port", &port, false, "", "Server listen port")
	p.BindStr("f", "format", &format, false, "json,csv,yaml", "Data output format")
	p.BindStr("", "token", &token, false, "", "Authentication token")
	p.BindBool("e", "encrypt", &encrypt, "Enable encryption")
	p.BindBool("", "plain", &plain, "Force unencrypted plain text")
	p.BindStr("k", "key", &key, false, "", "Encryption secret key")

	// 3. Document individual choices
	p.SetChoiceHelp("format", "json", "Structured JSON format")
	p.SetChoiceHelp("format", "csv", "Comma-separated values")
	p.SetChoiceHelp("format", "yaml", "Human-readable YAML")

	// 4. Validation rules
	p.AddExclusive("encrypt, plain") // Cannot supply both
	p.AddDependency("key", "encrypt") // --key requires --encrypt

	// 5. Positional arguments
	p.BindPos("input", &inputFile, true, "", "Primary input file")
	p.BindRest("files", &extraFiles, false, "", "Additional files to process")

	// 6. Parse arguments (pass os.Args[1:])
	if err := p.Parse(os.Args[1:]); err != nil {
		// Clean exit on help or version request
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			os.Exit(0)
		}

		// Handle specific validation errors
		switch {
		case errors.Is(err, goargv.ErrMutualExclusion):
			fmt.Fprintf(os.Stderr, "Conflict: %v\n", err)
		case errors.Is(err, goargv.ErrDependency):
			fmt.Fprintf(os.Stderr, "Dependency error: %v\n", err)
		case errors.Is(err, goargv.ErrMissingRequired):
			fmt.Fprintf(os.Stderr, "Missing required argument: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Processing %s on port %d (dry-run: %v, compress: %v)\n",
		inputFile, port, dryRun, compress)
}
```

---

## Supported Syntax

| Syntax | Example | Description |
|---|---|---|
| **No-dash binding** | `p.BindBool("v", "verbose", ...)` | Accepts options with or without leading hyphens. |
| **Standard flags** | `-f json`, `--format json` | Traditional space-separated flag values. |
| **Inline values** | `--format=json`, `--port=8080` | Standard key-value syntax. |
| **Boolean negation** | `--no-dry-run`, `--dry-run=false` | Toggles or explicitly disables boolean flags. |
| **Short clusters** | `-vcd` | Equivalent to passing `-v -c -d`. |
| **Attached short values** | `-I/usr/include`, `-p8080` | Short flags with immediate attached values. |
| **Counters** | `-v`, `-vv`, `-vvv` | Increments an `int` counter for each occurrence. |
| **Negative numbers** | `--offset -12`, `-t -0.5` | Negative numbers are not mistaken for flags. |
| **Value choices** | `choices: "json,csv,yaml"` | Validates values against comma- or pipe-delimited choices. |
| **Documented choices** | `p.SetChoiceHelp(...)` | Displays indented descriptions for each allowed choice in help screens. |
| **Default hints** | `[default: 8080]` | Help screens automatically show active non-zero / true defaults. |
| **Environment fallback** | `DATAX_PORT=9000 ./app` | Automatically resolves missing flags via `SetEnv` or `SetEnvPrefix`. |
| **Exclusive groups** | `--encrypt --plain` *(rejected)* | Forbids using conflicting options defined in an exclusive group. |
| **Flag dependencies** | `--key 123` *(without `--encrypt`)* | Enforces prerequisites before allowing a dependent option. |
| **Rest arguments** | `file.txt a.go b.go` | Trailing positional arguments collected into `*[]string`. |
| **Double-dash `--`** | `-- -weird-file.txt` | Treats everything following `--` as a positional argument. |

---

## API Reference

### Initialization & Configuration

```go
func New(programName string) *Parser
```
Creates a new parser instance.

```go
func (p *Parser) SetOutput(w io.Writer)
```
Redirects output (default `os.Stdout`) to any `io.Writer` (e.g. `bytes.Buffer` for unit tests).

```go
func (p *Parser) SetEnvPrefix(prefix string)
```
Sets a global prefix for environment variables. Any unset long flag will automatically look up `<PREFIX>_<FLAG_NAME>` (e.g., `--dry-run` checks `PREFIX_DRY_RUN`).

```go
func (p *Parser) SetVersion(version, sh, extraInfo string)
```
Configures version string and optional short flag (`-V`).

---

### Binding Options

Options accept names with or without leading hyphens. Passing a `nil` target pointer panics immediately with a clear error.

```go
func (p *Parser) BindBool(sh, lng string, target *bool, help string)
func (p *Parser) BindCount(sh, lng string, target *int, help string)
func (p *Parser) BindChar(sh, lng string, target *byte, required bool, choices, help string)
func (p *Parser) BindInt(sh, lng string, target *int, required bool, choices, help string)
func (p *Parser) BindFloat(sh, lng string, target *float64, required bool, choices, help string)
func (p *Parser) BindStr(sh, lng string, target *string, required bool, choices, help string)
```

#### Positional Arguments
```go
func (p *Parser) BindPos(name string, target *string, required bool, choices, help string)
func (p *Parser) BindRest(name string, target *[]string, required bool, choices, help string)
```

---

### Rules & Choices

```go
func (p *Parser) SetChoiceHelp(flagOrName, choice, help string) error
func (p *Parser) SetEnv(flagOrName, envVar string)
func (p *Parser) AddExclusive(flagsGroup string)
func (p *Parser) AddDependency(flag, requiresFlags string)
```

---

### Parsing & Error Sentinels

```go
func (p *Parser) Parse(args []string) error
```

`Parse` expects CLI arguments (typically `os.Args[1:]`). It returns typed sentinel errors checkable via `errors.Is`:
- `goargv.ErrHelpRequested`: Triggered by `-h` or `--help`.
- `goargv.ErrVersionRequested`: Triggered by `-V` or `--version`.
- `goargv.ErrUnknownFlag`: An unregistered flag or unexpected positional argument was passed.
- `goargv.ErrMissingValue`: A flag requiring an argument was left empty.
- `goargv.ErrInvalidValue`: Value conversion failed, choice violated, or invalid boolean value passed.
- `goargv.ErrMissingRequired`: A required flag or positional argument was omitted.
- `goargv.ErrMutualExclusion`: Conflicting options from an exclusive group were supplied.
- `goargv.ErrDependency`: A dependent flag was supplied without its required dependencies.

---

## Built-in Help System

`goargv` automatically aligns columns and formats boolean negations, default values, and environment variables:

```text
datax v1.2.0

Usage: datax [OPTIONS] <input> [files...]

Options:
  -h, --help                    Display this help message
  -V, --version                 Display version information
  -d, --[no-]dry-run            Simulate execution without modifying files
  -c, --[no-]compress           Enable payload compression [default: true]
  -v, --verbose                 Increase verbosity level (-v, -vv, -vvv)
  -m, --mode {r,w,a}            File access mode
  -p, --port <int>              Server listen port [default: 8080] [env: DATAX_PORT]
  -f, --format {json,csv,yaml}  Data output format [default: json]
                                  json: Structured JSON format
                                  csv: Comma-separated values
                                  yaml: Human-readable YAML
      --token <str>             Authentication token [env: DATAX_TOKEN]
  -e, --[no-]encrypt            Enable encryption
      --[no-]plain              Force unencrypted plain text
  -k, --key <str>               Encryption secret key

Positional Arguments:
  <input>                       Primary input file [required]
  <files...>                    Additional files to process
```

Inspect individual flags via `--help <flag_name>`:
```text
Short flag:            -p
Long flag:             --port
Value required:        Yes
Mandatory:             No
Default:               8080
Environment variable:  DATAX_PORT

Description:
  Server listen port
```
