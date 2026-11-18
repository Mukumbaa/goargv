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

## Full example
```go
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Mukumbaa/goargv"
)

type globalOpts struct {
	quiet     bool
	verbosity int
	context   string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var (
		global  globalOpts
		command string
	)

	root := goargv.New("svcctl")
	root.SetOutput(os.Stdout)
	root.SetVersion("1.4.0", "V", "Build 2026.11 (Go)")
	root.SetDescription("Service deployment and operations CLI.")
	root.SetEpilog("Run 'svcctl <command> --help' for command-specific options.")
	root.SetHelpWidth(60)
	root.SetEnvPrefix("SVCCTL")

	// Stop parsing global flags once the subcommand positional is reached
	root.SetStopAtPositional(true)

	// Global options
	root.BindBool("q", "quiet", &global.quiet, "Suppress informational output")
	root.BindCount("v", "verbose", &global.verbosity, "Increase log verbosity (-v, -vv)")
	root.BindStr("C", "context", &global.context, false, "", "Target cluster context")

	// Subcommand selector
	root.BindPos("command", &command, true, "deploy,logs,config", "Subcommand to execute")

	if err := checkConfigErrs(
		root.SetPlaceholder("context", "name"),
		root.SetDefault("context", "default"),
		root.SetChoiceHelp("command", "deploy", "Deploy a service and expose ports"),
		root.SetChoiceHelp("command", "logs", "Inspect or stream service logs"),
		root.SetChoiceHelp("command", "config", "Read or update CLI configuration"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "svcctl configuration error: %v\n", err)
		return 1
	}

	if err := root.Parse(args); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "svcctl: %v\n", err)
		if errors.Is(err, goargv.ErrMissingRequired) {
			fmt.Fprintln(os.Stderr)
			root.SetOutput(os.Stderr)
			root.PrintHelp()
		}
		return 1
	}

	rem := root.RemainingArgs()
	switch command {
	case "deploy":
		return cmdDeploy(global, rem)
	case "logs":
		return cmdLogs(global, rem)
	case "config":
		return cmdConfig(global, rem)
	default:
		return 1
	}
}

func cmdDeploy(g globalOpts, args []string) int {
	var (
		dryRun       bool
		healthCheck  = true // Default true -> displays --[no-]health-check in help
		mode         byte
		port         int
		offset       int
		traffic      float64
		strategy     string
		outputPath   string
		encrypt      bool
		plain        bool
		keyFile      string
		debugHooks   bool
		envVars      []string
		exposedPorts []int
		weights      []float64
		service      string
		artifacts    []string
	)

	p := goargv.New("svcctl deploy")
	p.SetOutput(os.Stdout)
	p.SetVersion("1.4.0", "V", "Subcommand: deploy")
	p.SetDescription("Deploy a service bundle to the selected cluster context.")
	p.SetEpilog("Examples:\n  svcctl -vv deploy --plain --no-health-check -p 9090 -E ENV=prod -P 80 -P 443 api ./dist.tar.gz")
	p.SetHelpWidth(0) // 0 = auto-detect from COLUMNS or fallback to 50
	p.SetEnvPrefix("SVCCTL_DEPLOY")

	// Scalar options
	p.BindBool("d", "dry-run", &dryRun, "Simulate deployment without applying changes")
	p.BindBool("H", "health-check", &healthCheck, "Wait for readiness probes after rollout")
	p.BindChar("m", "mode", &mode, false, "r,b,c", "Rollout mode")
	p.BindInt("p", "port", &port, false, "", "Primary container listen port")
	p.BindInt("O", "offset", &offset, false, "", "Canary skew offset (allows negative values)")
	p.BindFloat("t", "traffic", &traffic, false, "", "Initial traffic ratio (0.0 - 1.0)")
	p.BindStr("s", "strategy", &strategy, false, "fast,safe,canary", "Deployment strategy")
	p.BindStr("o", "output", &outputPath, false, "", "Manifest output path ('-' for stdout)")

	// Grouped security options & hidden diagnostic flag
	p.BindBool("e", "encrypt", &encrypt, "Encrypt bundle in transit")
	p.BindBool("", "plain", &plain, "Disable transport encryption")
	p.BindStr("k", "key-file", &keyFile, false, "", "TLS private key path")
	p.BindBool("D", "debug-hooks", &debugHooks, "Enable internal lifecycle hook tracing")

	// Repeatable list options
	p.BindStrList("E", "env", &envVars, false, "", "Environment variables (KEY=VAL)")
	p.BindIntList("P", "expose", &exposedPorts, false, "", "Additional ports to expose")
	p.BindFloatList("w", "weight", &weights, false, "", "Traffic weights per replica pool")

	// Cross-flag validation rules:
	// Combining AddExclusive + AddAnyOf enforces "exactly one" transport security flag
	p.AddExclusive("encrypt, plain")
	p.AddAnyOf("encrypt, plain")
	p.AddDependency("key-file", "encrypt")

	// Positionals: bind all scalar positionals before any rest argument
	p.BindPos("service", &service, true, "", "Target service name")
	p.BindRest("artifacts", &artifacts, false, "", "Optional bundle files")

	if err := checkConfigErrs(
		p.SetPlaceholder("output", "path"),
		p.SetPlaceholder("key-file", "pem"),
		p.SetPlaceholder("env", "K=V"),
		p.SetPlaceholder("artifacts", "file"),
		p.SetHelpGroup("encrypt", "Security Options"),
		p.SetHelpGroup("plain", "Security Options"),
		p.SetHelpGroup("key-file", "Security Options"),
		p.SetEnv("key-file", "DEPLOY_TLS_KEY"),
		p.SetHidden("debug-hooks"),
		p.SetDefault("dry-run", "false"),
		p.SetDefault("health-check", "true"),
		p.SetDefault("mode", "r"),
		p.SetDefault("port", "8080"),
		p.SetDefault("offset", "0"),
		p.SetDefault("traffic", "1.0"),
		p.SetDefault("strategy", "safe"),
		p.SetDefault("output", "-"),
		p.SetChoiceHelp("mode", "r", "Rolling update"),
		p.SetChoiceHelp("mode", "b", "Blue-green swap"),
		p.SetChoiceHelp("mode", "c", "Canary progressive"),
		p.SetChoiceHelp("strategy", "fast", "Minimize rollout time"),
		p.SetChoiceHelp("strategy", "safe", "Verify health checks between batches"),
		p.SetChoiceHelp("strategy", "canary", "Shift traffic gradually"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "deploy configuration error: %v\n", err)
		return 1
	}

	if err := p.Parse(args); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			return 0
		}
		switch {
		case errors.Is(err, goargv.ErrUnknownFlag):
			fmt.Fprintf(os.Stderr, "deploy: unrecognized option: %v\n", err)
			return 2
		case errors.Is(err, goargv.ErrMutualExclusion):
			fmt.Fprintf(os.Stderr, "deploy: conflicting flags: %v\n", err)
		case errors.Is(err, goargv.ErrDependency):
			fmt.Fprintf(os.Stderr, "deploy: missing dependency: %v\n", err)
		case errors.Is(err, goargv.ErrMissingRequired):
			fmt.Fprintf(os.Stderr, "deploy: missing required argument: %v\n", err)
		case errors.Is(err, goargv.ErrMissingValue), errors.Is(err, goargv.ErrInvalidValue):
			fmt.Fprintf(os.Stderr, "deploy: invalid input: %v\n", err)
		default:
			fmt.Fprintf(os.Stderr, "deploy: %v\n", err)
		}
		return 1
	}

	// Best practice: use IsSet to distinguish explicit CLI/env values from defaults
	portSource := "default"
	if p.IsSet("port") {
		portSource = "explicit"
	}

	if !g.quiet {
		fmt.Printf("[context=%s verbosity=%d] Deploying service %q\n", g.context, g.verbosity, service)
		fmt.Printf("  dry_run=%v health_check=%v mode=%c port=%d (%s) offset=%d traffic=%.2f strategy=%s output=%s\n",
			dryRun, healthCheck, mode, port, portSource, offset, traffic, strategy, outputPath)
		fmt.Printf("  encrypt=%v plain=%v key_file=%q debug_hooks=%v\n",
			encrypt, plain, keyFile, debugHooks)
		fmt.Printf("  env=%v exposed_ports=%v weights=%v artifacts=%v\n",
			envVars, exposedPorts, weights, artifacts)
	}

	return 0
}

func cmdLogs(g globalOpts, args []string) int {
	var (
		follow  bool
		lines   int
		level   string
		since   string
		filters []string
		service string
	)

	p := goargv.New("svcctl logs")
	p.SetDescription("Fetch or stream logs from a running service.")
	p.SetEnvPrefix("SVCCTL_LOGS")

	p.BindBool("f", "follow", &follow, "Follow log output")
	p.BindInt("n", "lines", &lines, false, "", "Number of recent lines to show")
	p.BindStr("l", "level", &level, false, "debug,info,warn,error", "Minimum log severity")
	p.BindStr("s", "since", &since, false, "", "Show logs newer than duration")
	p.BindStrList("F", "filter", &filters, false, "", "Include only matching tags")
	p.BindPos("service", &service, true, "", "Target service name")

	if err := checkConfigErrs(
		p.SetPlaceholder("since", "duration"),
		p.SetPlaceholder("filter", "tag"),
		p.SetDefault("lines", "100"),
		p.SetDefault("level", "info"),
		p.SetDefault("since", "15m"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "logs configuration error: %v\n", err)
		return 1
	}

	if err := p.Parse(args); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "logs: %v\n", err)
		return 1
	}

	linesSource := "default"
	if p.IsSet("lines") {
		linesSource = "explicit"
	}

	if !g.quiet {
		fmt.Printf("[context=%s] Streaming logs for %q (follow=%v lines=%d (%s) level=%s since=%s filters=%v)\n",
			g.context, service, follow, lines, linesSource, level, since, filters)
	}

	return 0
}

func cmdConfig(g globalOpts, args []string) int {
	var (
		globalScope bool
		format      string
		action      string
		restArgs    []string
	)

	p := goargv.New("svcctl config")
	p.SetDescription("Inspect or modify CLI configuration profiles.")

	p.BindBool("g", "global", &globalScope, "Operate on global configuration")
	p.BindStr("f", "format", &format, false, "text,json", "Output serialization format")
	p.BindPos("action", &action, true, "get,set,list", "Configuration action")
	p.BindRest("args", &restArgs, false, "", "Action arguments (<key> or <key> <value>)")

	if err := checkConfigErrs(
		p.SetDefault("format", "text"),
		p.SetChoiceHelp("action", "get", "Read a configuration key"),
		p.SetChoiceHelp("action", "set", "Write a configuration key/value pair"),
		p.SetChoiceHelp("action", "list", "List all configuration entries"),
	); err != nil {
		fmt.Fprintf(os.Stderr, "config configuration error: %v\n", err)
		return 1
	}

	if err := p.Parse(args); err != nil {
		if errors.Is(err, goargv.ErrHelpRequested) || errors.Is(err, goargv.ErrVersionRequested) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	// Post-parse domain validation
	if action == "get" && len(restArgs) != 1 {
		fmt.Fprintln(os.Stderr, "config: 'get' requires exactly 1 argument (<key>)")
		return 1
	}
	if action == "set" && len(restArgs) != 2 {
		fmt.Fprintln(os.Stderr, "config: 'set' requires exactly 2 arguments (<key> <value>)")
		return 1
	}

	if !g.quiet {
		fmt.Printf("[context=%s] config action=%s global=%v format=%s args=%v\n",
			g.context, action, globalScope, format, restArgs)
	}

	return 0
}

func checkConfigErrs(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
```
