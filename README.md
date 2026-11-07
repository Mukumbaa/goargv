# libargv (Go)

A lightweight, zero-dependency CLI argument parser for Go. 

Designed as a modern, powerful alternative to Go's standard `flag` package, `libargv` allows you to bind command-line options and positional arguments directly to your pointers while supporting POSIX-compliant short flag clustering, value constraints, environment variable fallbacks, and multi-flag logical rules.

---

## Why use this over Go's standard `flag`?

Go's built-in `flag` package is reliable but lacks many features expected in modern CLI tools. `libargv` fills that gap without pulling in heavy external frameworks:

- **Zero dependencies**: Uses only the Go standard library (`errors`, `fmt`, `os`, `path/filepath`, `strconv`, `strings`).
- **Flexible flag definition**: Pass `"v"` and `"verbose"` or `"-v"` and `"--verbose"`—prefixes are automatically normalized.
- **Short flag clustering**: Bundles short flags seamlessly (e.g., `-xvf archive.tar.gz` or `-vvv`).
- **Allowed choices validation**: Restrict valid inputs using comma- or pipe-separated strings (e.g., `"fast,safe,custom"`).
- **Environment variable fallback**: Automatically read unset flags from environment variables (`SetEnv`).
- **Logical relationships**: Enforce mutual exclusion across groups (`AddExclusive`) and define prerequisite flag dependencies (`AddDependency`).
- **First-class positional & rest arguments**: Easily declare mandatory/optional positional arguments and trailing slices (`BindRest`).
- **Idiomatic error handling**: Inspect failures using standard `errors.Is(err, libargv.ErrMutualExclusion)`.
- **Intelligent help**: Auto-aligned options with choices and environment variable previews, plus per-flag inspection (e.g. `--help port`).

---

## Installation

```bash
go get github.com/yourusername/libargv
```

*(Replace `yourusername` with your actual GitHub username or repository path)*

---

## Quick Example

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/yourusername/libargv"
)

func main() {
	p := libargv.New("converter")

	// 1. Declare target variables with default values
	var (
		verbose  int
		jsonFmt  bool
		xmlFmt   bool
		iniFmt   bool
		port     int = 8080
		user     string
		pass     string
		input    string
		restArgs []string
	)

	// 2. Bind flags (prefixes '-' and '--' are optional!)
	p.BindCount("v", "verbose", &verbose, "Increase verbosity level (-v, -vv)")
	p.BindBool("j", "json", &jsonFmt, "Output as JSON")
	p.BindBool("x", "xml", &xmlFmt, "Output as XML")
	p.BindBool("i", "ini", &iniFmt, "Output as INI")

	p.BindInt("p", "port", &port, false, "", "Server listen port")
	p.BindStr("u", "user", &user, false, "", "Admin username")
	p.BindStr("", "pass", &pass, false, "", "Admin password")

	// 3. Positional and rest arguments
	p.BindPos("input", &input, true, "", "Source input file")
	p.BindRest("files", &restArgs, false, "", "Additional files")

	// 4. Advanced configuration:
	// - Read $APP_PORT if --port is omitted
	p.SetEnv("port", "APP_PORT")

	// - Only one output format allowed
	p.AddExclusive("json,xml,ini")

	// - Setting --pass requires --user to be provided as well
	p.AddDependency("pass", "user")

	// 5. Parse command line arguments (os.Args[1:])
	if err := p.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, libargv.ErrHelpRequested) {
			os.Exit(0)
		}

		// Handle specific validation errors
		switch {
		case errors.Is(err, libargv.ErrMutualExclusion):
			fmt.Fprintf(os.Stderr, "Conflict error: %v\n", err)
		case errors.Is(err, libargv.ErrDependency):
			fmt.Fprintf(os.Stderr, "Dependency error: %v\n", err)
		case errors.Is(err, libargv.ErrMissingRequired):
			fmt.Fprintf(os.Stderr, "Missing argument: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Port: %d | User: %s | Verbosity: %d | File: %s\n", port, user, verbose, input)
	if len(restArgs) > 0 {
		fmt.Printf("Additional files: %v\n", restArgs)
	}
}
```

---

## Supported Syntax

| Syntax | Example | Description |
|---|---|---|
| **No-dash binding** | `p.BindBool("v", "verbose", ...)` | Accepts options with or without leading hyphens. |
| **Standard flags** | `-f json`, `--format json` | Traditional space-separated flag values. |
| **Inline values** | `--format=json` | Long flags with inline values. |
| **Short clusters** | `-vxl` | Equivalent to passing `-v -x -l`. |
| **Attached short values** | `-I/usr/include`, `-p8080` | Values attached directly to the short flag. |
| **Counters** | `-v`, `-vv`, `-vvv` | Increments an `int` counter for each occurrence. |
| **Negative numbers** | `--offset -12`, `-t -0.5` | Negative numbers are not mistaken for flags. |
| **Value choices** | `choices: "json,xml"` | Validates values against comma- or pipe-delimited choices. |
| **Environment fallback** | `APP_PORT=9000 ./app` | Uses environment variable value when flag is omitted. |
| **Exclusive groups** | `--json --xml` *(rejected)* | Forbids using options from the same exclusive group together. |
| **Flag dependencies** | `--pass 123` *(without `--user`)* | Enforces prerequisites before allowing a dependent option. |
| **Rest arguments** | `file.txt a.go b.go` | Variadic trailing arguments collected into `*[]string`. |
| **Double-dash `--`** | `-- -weird-file.txt` | Treats everything following `--` as a positional argument. |

---

## API Reference

### Initialization

```go
func New(programName string) *Parser
```
Creates a new `Parser` instance.

### Binding Options

Options accept names with or without leading dashes (e.g. `"v"` or `"-v"`, `"verbose"` or `"--verbose"`).

```go
func (p *Parser) BindBool(sh, lng string, target *bool, help string)
func (p *Parser) BindCount(sh, lng string, target *int, help string)
func (p *Parser) BindChar(sh, lng string, target *byte, required bool, choices, help string)
func (p *Parser) BindInt(sh, lng string, target *int, required bool, choices, help string)
func (p *Parser) BindFloat(sh, lng string, target *float64, required bool, choices, help string)
func (p *Parser) BindStr(sh, lng string, target *string, required bool, choices, help string)
```

### Positional Arguments

```go
func (p *Parser) BindPos(name string, target *string, required bool, choices, help string)
func (p *Parser) BindRest(name string, target *[]string, required bool, choices, help string)
```

### Advanced Rules & Environment Variables

```go
func (p *Parser) SetEnv(flagOrName, envVar string)
func (p *Parser) AddExclusive(flagsGroup string)
func (p *Parser) AddDependency(flag, requiresFlags string)
```

### Parsing & Error Sentinels

```go
func (p *Parser) Parse(args []string) error
```

`Parse` returns typed sentinel errors checkable via `errors.Is`:
- `libargv.ErrHelpRequested`: Triggered by `-h` or `--help`. Help text is automatically printed.
- `libargv.ErrUnknownFlag`: An unregistered flag or unexpected positional argument was passed.
- `libargv.ErrMissingValue`: A flag requiring a value was left empty.
- `libargv.ErrInvalidValue`: Value parsing failed (e.g., invalid integer) or violated `choices`.
- `libargv.ErrMissingRequired`: A required flag or positional argument was not provided.
- `libargv.ErrMutualExclusion`: Multiple flags from an exclusive group were supplied.
- `libargv.ErrDependency`: A dependent flag was provided without satisfying its required dependencies.

