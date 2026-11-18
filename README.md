# goargv

A zero-dependency command-line argument parser for Go with direct pointer binding, environment variable resolution, and validation rules.

---

## Features

- **Standard library only**: Built strictly with Go standard library packages.
- **Direct variable binding**: Binds directly to `*bool`, `*byte`, `*int`, `*float64`, `*string`, `*[]string`, `*[]int`, and `*[]float64`.
- **Explicit defaults**: Set initial variable values and help-screen tags via `SetDefault`.
- **Explicit presence check**: Query whether an argument was supplied via CLI or environment variable using `IsSet`.
- **Validation**: Value whitelists (`choices`), mutual exclusion groups (`AddExclusive`), at-least-one groups (`AddAnyOf`), required flag dependencies (`AddDependency`), and duplicate flag detection at bind time.
- **POSIX & GNU syntax**: Short-flag clustering (`-xvf`), inline assignments (`--port=8080`), attached values (`-p8080`, `-p=8080`), negative numbers (`--offset -12`), and stdin/stdout markers (`-o -`).
- **Standard `--` delimiter**: Unambiguously terminates flag parsing.
- **Environment variables**: Automatic prefix-based resolution (`APP_PORT`) with support for explicit overrides.
- **Typed sentinel errors**: Check failures with standard `errors.Is(err, goargv.ErrMutualExclusion)`.
- **Repeatable flags**: Collect repeated option occurrences into slices via `BindStrList`, `BindIntList`, and `BindFloatList`.
- **Clean subcommands**: Stop flag/help parsing at the first positional argument via `SetStopAtPositional` and forward `RemainingArgs()` to sub-parsers.
- **Rich help customization**: Program description (`SetDescription`), epilog (`SetEpilog`), grouped option sections (`SetHelpGroup`), custom placeholders (`SetPlaceholder`), configurable/auto `COLUMNS` wrap width (`SetHelpWidth`), and hidden flags (`SetHidden`).
- **Redirectable output**: Inject an arbitrary `io.Writer` via `SetOutput` for headless unit tests.

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

	var (
		dryRun     bool
		compress   bool
		verbosity  int
		port       int
		offset     int
		mode       byte
		format     string
		output     string
		encrypt    bool
		plain      bool
		key        string
		inputFile  string
		extraFiles []string
	)

	p.SetVersion("1.2.0", "V", "Build 2026.1")
	p.SetDescription("Fast data transformation and encryption utility.")
	p.SetEpilog("Examples:\n  datax -f json -o out.json input.csv")
	p.SetEnvPrefix("APP")

	// Option flags
	p.BindBool("d", "dry-run", &dryRun, "Simulate execution")
	p.BindBool("c", "compress", &compress, "Enable compression")
	p.BindCount("v", "verbose", &verbosity, "Increase verbosity level (-v, -vv)")
	p.BindInt("p", "port", &port, false, "", "Server listen port")
	p.BindInt("", "offset", &offset, false, "", "Memory offset delta")
	p.BindChar("m", "mode", &mode, false, "r,w,a", "File access mode")
	p.BindStr("f", "format", &format, false, "json,csv,yaml", "Data format")
	p.BindStr("o", "output", &output, false, "", "Output destination ('-' for stdout)")
	p.BindBool("e", "encrypt", &encrypt, "Encrypt payload")
	p.BindBool("", "plain", &plain, "Disable encryption")
	p.BindStr("k", "key", &key, false, "", "Encryption key")

	// Custom placeholders & help groups
	_ = p.SetPlaceholder("output", "path")
	_ = p.SetHelpGroup("encrypt", "Security Options")
	_ = p.SetHelpGroup("plain", "Security Options")
	_ = p.SetHelpGroup("key", "Security Options")

	// Defaults (sets target value and registers help text)
	_ = p.SetDefault("dry-run", "false")
	_ = p.SetDefault("compress", "true")
	_ = p.SetDefault("port", "8080")
	_ = p.SetDefault("offset", "0")
	_ = p.SetDefault("mode", "r")
	_ = p.SetDefault("format", "json")
	_ = p.SetDefault("output", "-")

	// Choice documentation
	_ = p.SetChoiceHelp("format", "json", "Structured JSON format")
	_ = p.SetChoiceHelp("format", "csv", "Comma-separated values")
	_ = p.SetChoiceHelp("format", "yaml", "Human-readable YAML")

	// Rules
	p.AddExclusive("encrypt, plain")
	p.AddDependency("key", "encrypt")

	// Positional arguments (scalar positionals must be bound before rest arguments)
	p.BindPos("input", &inputFile, true, "", "Input file path")
	p.BindRest("files", &extraFiles, false, "", "Additional files")

	if err := p.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			os.Exit(0)
		}

		switch {
		case errors.Is(err, goargv.ErrMutualExclusion):
			fmt.Fprintf(os.Stderr, "Flag conflict: %v\n", err)
		case errors.Is(err, goargv.ErrDependency):
			fmt.Fprintf(os.Stderr, "Missing dependency: %v\n", err)
		case errors.Is(err, goargv.ErrMissingRequired):
			fmt.Fprintf(os.Stderr, "Missing required argument: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Input: %s, Port: %d (explicit: %v), Mode: %c, Output: %s\n",
		inputFile, port, p.IsSet("port"), mode, output)
}
```

---

## Supported Syntax

| Syntax | Example | Description |
|---|---|---|
| No-dash binding | `p.BindBool("v", "verbose", ...)` | Accepts options with or without leading hyphens. |
| Standard flags | `-p 8080`, `--port 8080` | Space-separated flag values. |
| Inline values | `--port=8080` | Key-value flag syntax. |
| Boolean negation | `--no-compress`, `--compress=false` | Disables a boolean switch. |
| Short clusters | `-xvf` | Expands to `-x -v -f`. |
| Attached values | `-p8080`, `-p=8080`, `-I/usr/include` | Value attached directly to the short flag. |
| Counter flags | `-v`, `-vv`, `-vvv` | Increments an `int` counter. |
| Negative numbers | `--offset -12` | Negative numeric literals are not parsed as flags. |
| Stdin/stdout flag | `-o -` | Preserves `-` as a literal value. |
| Delimiter | `-- -file.txt` | Treats everything after `--` as positional arguments. |

---

## Subcommands

Multi-command CLIs can enable `SetStopAtPositional(true)` on the root parser so global flags are parsed up to the subcommand name, and all trailing flags (including `--help`) are forwarded intact via `RemainingArgs()`:

```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Mukumbaa/goargv"
)

func main() {
	var (
		verbose bool
		cmd     string
	)

	root := goargv.New("git")
	root.SetStopAtPositional(true)
	root.BindBool("v", "verbose", &verbose, "Enable verbose output")
	root.BindPos("command", &cmd, true, "commit,clone", "Subcommand to run")
	_ = root.SetChoiceHelp("command", "commit", "Record repository changes")
	_ = root.SetChoiceHelp("command", "clone", "Clone remote repository")

	if err := root.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "git: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "commit":
		runCommit(verbose, root.RemainingArgs())
	}
}

func runCommit(verbose bool, args []string) {
	var message string
	p := goargv.New("git commit")
	p.BindStr("m", "message", &message, true, "", "Commit message")

	if err := p.Parse(args); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "commit: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[verbose=%v] Committed: %s\n", verbose, message)
}
```

---

## API Reference

### Initialization & Configuration

```go
func New(programName string) *Parser
func (p *Parser) SetOutput(w io.Writer)
func (p *Parser) SetEnvPrefix(prefix string)
func (p *Parser) SetVersion(version, sh, extraInfo string)
func (p *Parser) SetDescription(desc string)
func (p *Parser) SetEpilog(epilog string)
func (p *Parser) SetHelpWidth(width int)
func (p *Parser) SetStopAtPositional(stop bool)
```

### Option Binding

```go
func (p *Parser) BindBool(sh, lng string, target *bool, help string)
func (p *Parser) BindCount(sh, lng string, target *int, help string)
func (p *Parser) BindChar(sh, lng string, target *byte, required bool, choices, help string)
func (p *Parser) BindInt(sh, lng string, target *int, required bool, choices, help string)
func (p *Parser) BindFloat(sh, lng string, target *float64, required bool, choices, help string)
func (p *Parser) BindStr(sh, lng string, target *string, required bool, choices, help string)

// Note: BindRest must be called after all scalar positional arguments
func (p *Parser) BindPos(name string, target *string, required bool, choices, help string)
func (p *Parser) BindRest(name string, target *[]string, required bool, choices, help string)
func (p *Parser) BindStrList(sh, lng string, target *[]string, required bool, choices, help string)
func (p *Parser) BindIntList(sh, lng string, target *[]int, required bool, choices, help string)
func (p *Parser) BindFloatList(sh, lng string, target *[]float64, required bool, choices, help string)
```

### Defaults, Rules & Documentation

```go
func (p *Parser) SetDefault(flagOrName, defaultVal string) error
func (p *Parser) SetEnv(flagOrName, envVar string) error
func (p *Parser) SetChoiceHelp(flagOrName, choice, help string) error
func (p *Parser) SetPlaceholder(flagOrName, placeholder string) error
func (p *Parser) SetHelpGroup(flagOrName, group string) error
func (p *Parser) SetHidden(flagOrName string) error
func (p *Parser) AddExclusive(flagsGroup string)
func (p *Parser) AddDependency(flag, requiresFlags string)
func (p *Parser) AddAnyOf(flagsGroup string)
```

### Parsing

```go
func (p *Parser) Parse(args []string) error
func (p *Parser) IsSet(flagOrName string) bool
func (p *Parser) RemainingArgs() []string
func (p *Parser) PrintHelp()
```

**Sentinel errors:**
- `goargv.ErrHelpRequested`
- `goargv.ErrVersionRequested`
- `goargv.ErrUnknownFlag`
- `goargv.ErrMissingValue`
- `goargv.ErrInvalidValue`
- `goargv.ErrMissingRequired`
- `goargv.ErrMutualExclusion`
- `goargv.ErrDependency`
