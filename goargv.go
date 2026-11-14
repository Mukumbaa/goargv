package goargv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	ErrHelpRequested    = errors.New("help requested")
	ErrVersionRequested = errors.New("version requested")
	ErrUnknownFlag      = errors.New("unknown flag")
	ErrMissingValue     = errors.New("missing value")
	ErrInvalidValue     = errors.New("invalid value")
	ErrMissingRequired  = errors.New("missing required argument")
	ErrMutualExclusion  = errors.New("mutually exclusive flags")
	ErrDependency       = errors.New("flag dependency not satisfied")
)

type CliType int

const (
	TypeBool CliType = iota
	TypeCount
	TypeChar
	TypeInt
	TypeFloat
	TypeStr
	TypeRest
	TypeStrList
)

type ruleType int

const (
	ruleExclusive ruleType = iota
	ruleDependency
)

type cliRule struct {
	ruleType ruleType
	target   string
	requires string
}

type cliChoiceHelp struct {
	choice string
	help   string
}

type cliArg struct {
	name         string
	shortFlag    string
	longFlag     string
	help         string
	choices      string
	choiceHelps  []cliChoiceHelp
	envVar       string
	defaultVal   string
	hasDefault   bool
	defaultBool  bool
	isHidden     bool
	argType      CliType
	target       any
	isPositional bool
	required     bool
	isSet        bool
}

type Parser struct {
	programName  string
	version      string
	versionShort string
	versionExtra string
	envPrefix    string
	out          io.Writer
	args         []*cliArg
	rules        []cliRule
	initErr      error
}

// New creates and initializes an argument parser for the given program name
func New(programName string) *Parser {
	return &Parser{
		programName: programName,
		out:         os.Stdout,
		args:        make([]*cliArg, 0, 8),
		rules:       make([]cliRule, 0),
	}
}

// SetOutput overrides the destination writer for help and version output
func (p *Parser) SetOutput(w io.Writer) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if w != nil {
		p.out = w
	}
}

// SetEnvPrefix sets a global prefix used to infer environment variable names for long flags
func (p *Parser) SetEnvPrefix(prefix string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.envPrefix = strings.TrimSpace(prefix)
}

// getEffectiveEnv returns the environment variable bound to an argument, falling back to prefix+flag
func (p *Parser) getEffectiveEnv(a *cliArg) string {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if a.envVar != "" {
		return a.envVar
	}
	if p.envPrefix == "" || a.longFlag == "" {
		return ""
	}
	clean := strings.TrimPrefix(a.longFlag, "--")
	clean = strings.ReplaceAll(clean, "-", "_")
	return strings.ToUpper(p.envPrefix + "_" + clean)
}

// SetDefault sets an explicit default value, assigns it to the bound target, and formats it in help
func (p *Parser) SetDefault(flagOrName, defaultVal string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found for default value", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	if err := p.assignVal(arg, defaultVal, flagOrName); err != nil {
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	arg.defaultVal = defaultVal
	arg.hasDefault = true
	if arg.argType == TypeBool {
		arg.defaultBool = *(arg.target.(*bool))
	}
	arg.isSet = false
	return nil
}

// normalizeShort ensures that a short flag begins with a single hyphen
func normalizeShort(sh string) string {
	sh = strings.TrimSpace(sh)
	if sh == "" {
		return ""
	}
	if strings.HasPrefix(sh, "-") {
		return sh
	}
	return "-" + sh
}

// normalizeLong ensures that a long flag begins with double hyphens
func normalizeLong(lng string) string {
	lng = strings.TrimSpace(lng)
	if lng == "" {
		return ""
	}
	if strings.HasPrefix(lng, "--") {
		return lng
	}
	if strings.HasPrefix(lng, "-") {
		return "-" + lng
	}
	return "--" + lng
}

// displayName returns the most descriptive label available for this argument
func (a *cliArg) displayName() string {
	if a.longFlag != "" {
		return a.longFlag
	}
	if a.shortFlag != "" {
		return a.shortFlag
	}
	if a.name != "" {
		return a.name
	}
	return "argument"
}

// addArg registers an argument definition and checks that its target pointer is valid
func (p *Parser) addArg(name, sh, lng string, t CliType, target any, required, isPos bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if target == nil {
		displayName := lng
		if displayName == "" {
			displayName = sh
		}
		if displayName == "" {
			displayName = name
		}
		panic(fmt.Sprintf("goargv: target pointer for argument '%s' cannot be nil", displayName))
	}
	if isPos {
		for _, a := range p.args {
			if a.isPositional && a.argType == TypeRest {
				panic(fmt.Sprintf("goargv: cannot bind positional argument '%s' after a rest argument", name))
			}
		}
	}
	var defBool bool
	if t == TypeBool {
		defBool = *(target.(*bool))
	}
	p.args = append(p.args, &cliArg{
		name:         name,
		shortFlag:    normalizeShort(sh),
		longFlag:     normalizeLong(lng),
		argType:      t,
		target:       target,
		defaultBool:  defBool,
		required:     required,
		isPositional: isPos,
		choices:      choices,
		choiceHelps:  make([]cliChoiceHelp, 0),
		help:         help,
	})
}

// BindBool binds a boolean flag and supports automatic --no- negation
func (p *Parser) BindBool(sh, lng string, target *bool, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeBool, target, false, false, "", help)
}

// BindCount binds an integer flag that increments each time the flag is supplied on the CLI
func (p *Parser) BindCount(sh, lng string, target *int, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeCount, target, false, false, "", help)
}

// BindChar binds a single-byte/character flag
func (p *Parser) BindChar(sh, lng string, target *byte, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeChar, target, required, false, choices, help)
}

// BindStr binds a string flag
func (p *Parser) BindStr(sh, lng string, target *string, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeStr, target, required, false, choices, help)
}

// BindInt binds an integer flag
func (p *Parser) BindInt(sh, lng string, target *int, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeInt, target, required, false, choices, help)
}

// BindFloat binds a 64-bit float flag
func (p *Parser) BindFloat(sh, lng string, target *float64, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg("", sh, lng, TypeFloat, target, required, false, choices, help)
}

// BindPos binds a positional string argument
func (p *Parser) BindPos(name string, target *string, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.addArg(name, "", "", TypeStr, target, required, true, choices, help)
}

// BindRest binds a slice to capture all remaining trailing positional arguments
func (p *Parser) BindRest(name string, target *[]string, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if target == nil {
		panic(fmt.Sprintf("goargv: target pointer for positional rest '%s' cannot be nil", name))
	}
	*target = nil
	p.addArg(name, "", "", TypeRest, target, required, true, choices, help)
}

// BindStrList binds a slice to collect repeated occurrences of a flag (e.g. -I dir1 -I dir2)
func (p *Parser) BindStrList(sh, lng string, target *[]string, required bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if target == nil {
		displayName := lng
		if displayName == "" {
			displayName = sh
		}
		panic(fmt.Sprintf("goargv: target pointer for flag '%s' cannot be nil", displayName))
	}
	*target = nil
	p.addArg("", sh, lng, TypeStrList, target, required, false, choices, help)
}

// findArg searches for an argument by identifier, matching against its name, long flag, or short flag
func (p *Parser) findArg(ident string) *cliArg {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	ident = strings.TrimSpace(ident)
	if ident == "" {
		return nil
	}
	for _, a := range p.args {
		if a.name != "" && a.name == ident {
			return a
		}
		if a.longFlag != "" {
			if a.longFlag == ident || strings.TrimPrefix(a.longFlag, "--") == ident {
				return a
			}
		}
		if a.shortFlag != "" {
			if a.shortFlag == ident || strings.TrimPrefix(a.shortFlag, "-") == ident {
				return a
			}
		}
	}
	return nil
}

// SetEnv binds a specific environment variable name to an existing argument or flag
func (p *Parser) SetEnv(flagOrName, envVar string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found to set environment variable", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	arg.envVar = envVar
	return nil
}

// SetHidden marks an argument as hidden so it does not appear in help screens
func (p *Parser) SetHidden(flagOrName string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found to set hidden", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	arg.isHidden = true
	return nil
}

// choiceExists reports whether a candidate value belongs to the argument's whitelist of choices
func (arg *cliArg) choiceExists(choice string) bool {
	if arg.choices == "" || choice == "" {
		return false
	}
	if arg.argType == TypeChar && !strings.ContainsAny(arg.choices, ",|") {
		return len(choice) == 1 && strings.Contains(arg.choices, choice)
	}

	tokens := splitTokens(arg.choices)
	for _, token := range tokens {
		if token == choice {
			return true
		}
	}
	return false
}

// SetChoiceHelp assigns documentation to an individual choice option belonging to an argument
func (p *Parser) SetChoiceHelp(flagOrName, choice, help string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found for choice help", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}

	if arg.choices == "" {
		err := fmt.Errorf("%w: argument '%s' has no choices defined", ErrInvalidValue, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}

	if !arg.choiceExists(choice) {
		err := fmt.Errorf("%w: choice '%s' is not valid for argument '%s' (allowed: [%s])",
			ErrInvalidValue, choice, arg.displayName(), arg.choices)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}

	for i := range arg.choiceHelps {
		if arg.choiceHelps[i].choice == choice {
			arg.choiceHelps[i].help = help
			return nil
		}
	}

	arg.choiceHelps = append(arg.choiceHelps, cliChoiceHelp{
		choice: choice,
		help:   help,
	})
	return nil
}

// AddExclusive defines a group of flags (separated by commas or pipes) that cannot be used together
func (p *Parser) AddExclusive(flagsGroup string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.rules = append(p.rules, cliRule{
		ruleType: ruleExclusive,
		target:   flagsGroup,
	})
}

// AddDependency declares that flag requires one or more other flags to be provided alongside it
func (p *Parser) AddDependency(flag, requiresFlags string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.rules = append(p.rules, cliRule{
		ruleType: ruleDependency,
		target:   flag,
		requires: requiresFlags,
	})
}

// splitTokens splits a string by comma or pipe delimiters into trimmed slices
func splitTokens(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '|'
	})
	var res []string
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			res = append(res, t)
		}
	}
	return res
}

// getChoiceTokens returns the list of allowed choice tokens, handling single-char strings without delimiters
func (arg *cliArg) getChoiceTokens() []string {
	if arg.choices == "" {
		return nil
	}
	if arg.argType == TypeChar && !strings.ContainsAny(arg.choices, ",|") {
		tokens := make([]string, 0, len(arg.choices))
		for i := 0; i < len(arg.choices); i++ {
			tokens = append(tokens, string(arg.choices[i]))
		}
		return tokens
	}
	return splitTokens(arg.choices)
}

// checkChoice validates if the assigned string fits within the argument's configured choice set
func (arg *cliArg) checkChoice(valStr string) bool {
	if arg.choices == "" {
		return true
	}
	if arg.argType == TypeChar && !strings.ContainsAny(arg.choices, ",|") {
		return len(valStr) == 1 && strings.Contains(arg.choices, valStr)
	}

	tokens := splitTokens(arg.choices)
	for _, token := range tokens {
		switch arg.argType {
		case TypeStr, TypeChar, TypeRest, TypeStrList:
			if token == valStr {
				return true
			}
		case TypeInt:
			cv, err1 := strconv.Atoi(token)
			vv, err2 := strconv.Atoi(valStr)
			if err1 == nil && err2 == nil && cv == vv {
				return true
			}
		case TypeFloat:
			cv, err1 := strconv.ParseFloat(token, 64)
			vv, err2 := strconv.ParseFloat(valStr, 64)
			if err1 == nil && err2 == nil && cv == vv {
				return true
			}
		}
	}
	return false
}

// countChoices returns the total number of allowed choices defined for the argument
func (arg *cliArg) countChoices() int {
	if arg.choices == "" {
		return 0
	}
	if arg.argType == TypeChar && !strings.ContainsAny(arg.choices, ",|") {
		return len(arg.choices)
	}
	return len(splitTokens(arg.choices))
}

// formatTypeHint renders the CLI usage placeholder or choice summary for the argument
func (arg *cliArg) formatTypeHint() string {
	if arg.choices != "" && len(arg.choiceHelps) < arg.countChoices() {
		return fmt.Sprintf(" {%s}", arg.choices)
	}
	switch arg.argType {
	case TypeStr:
		return " <str>"
	case TypeStrList:
		return " <str>..."
	case TypeInt:
		return " <int>"
	case TypeFloat:
		return " <num>"
	case TypeChar:
		return " <char>"
	case TypeRest:
		return "..."
	default:
		return ""
	}
}

// formatDefaultDisplay converte caratteri non visibili o spazi in sequenze leggibili per l'help
func formatDefaultDisplay(s string) string {
	if s == "" {
		return `""`
	}
	if s == " " {
		return `' '`
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatLongDisplay formats the long flag label, adding --[no-] annotations for boolean switches
func (a *cliArg) formatLongDisplay() string {
	if a.longFlag == "" {
		return ""
	}
	if a.argType == TypeBool && strings.HasPrefix(a.longFlag, "--") && !strings.HasPrefix(a.longFlag, "--no-") {
		if a.defaultBool {
			return "--[no-]" + strings.TrimPrefix(a.longFlag, "--")
		}
	}
	return a.longFlag
}

// printSingleHelp prints a detailed view of a single argument's configuration and choices
func (p *Parser) printSingleHelp(arg *cliArg) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	descWidth := 50

	effEnv := p.getEffectiveEnv(arg)

	if arg.isPositional {
		fmt.Fprintf(p.out, "%-23s<%s>\n", "Positional argument:", arg.displayName())
		fmt.Fprintf(p.out, "%-23s%v\n", "Mandatory:", arg.required)
	} else {
		sh := "None"
		if arg.shortFlag != "" {
			sh = arg.shortFlag
		}
		lng := "None"
		if arg.longFlag != "" {
			if arg.argType == TypeBool && strings.HasPrefix(arg.longFlag, "--") && !strings.HasPrefix(arg.longFlag, "--no-") && arg.defaultBool {
				lng = fmt.Sprintf("%s (negation: --no-%s)", arg.longFlag, strings.TrimPrefix(arg.longFlag, "--"))
			} else {
				lng = arg.longFlag
			}
		}
		fmt.Fprintf(p.out, "%-23s%s\n", "Short flag:", sh)
		fmt.Fprintf(p.out, "%-23s%s\n", "Long flag:", lng)
		valReq := arg.argType != TypeBool && arg.argType != TypeCount
		fmt.Fprintf(p.out, "%-23s%v\n", "Value required:", valReq)
		fmt.Fprintf(p.out, "%-23s%v\n", "Mandatory:", arg.required)
	}
	if arg.hasDefault {
		fmt.Fprintf(p.out, "%-23s%s\n", "Default:", formatDefaultDisplay(arg.defaultVal))
	}
	if effEnv != "" {
		fmt.Fprintf(p.out, "%-23s%s\n", "Environment variable:", effEnv)
	}
	if arg.choices != "" {
		fmt.Fprintf(p.out, "Allowed choices:\n")
		tokens := arg.getChoiceTokens()
		maxCLen := 0
		for _, tok := range tokens {
			if l := utf8.RuneCountInString(tok); l > maxCLen {
				maxCLen = l
			}
		}
		helpMap := make(map[string]string)
		for _, ch := range arg.choiceHelps {
			helpMap[ch.choice] = ch.help
		}

		for _, tok := range tokens {
			h := helpMap[tok]
			if h == "" {
				fmt.Fprintf(p.out, "  %s\n", tok)
			} else {
				pad := maxCLen - utf8.RuneCountInString(tok)
				prefix := fmt.Sprintf("  %s%*s:", tok, pad, "")
				printWrappedDesc(p.out, prefix, utf8.RuneCountInString(prefix), 1, h, descWidth)
			}
		}
	}
	fmt.Fprintf(p.out, "\nDescription:\n")
	help := "No documentation available."
	if arg.help != "" {
		help = arg.help
	}
	printWrappedDesc(p.out, "", 2, 0, help, descWidth)
}

// printWrappedDesc prints an indented label and word-wraps the accompanying description text
func printWrappedDesc(w io.Writer, leftCol string, indent int, gutter int, text string, descWidth int) {
	leftLen := utf8.RuneCountInString(leftCol)

	if leftLen > 0 {
		fmt.Fprint(w, leftCol)
		if leftLen < indent {
			fmt.Fprintf(w, "%*s", indent-leftLen+gutter, "")
		} else {
			fmt.Fprintf(w, "%*s", gutter, "")
		}
	} else {
		fmt.Fprintf(w, "%*s", indent+gutter, "")
	}

	if text == "" {
		fmt.Fprintln(w)
		return
	}

	contIndent := indent + gutter
	if leftLen > indent {
		contIndent = leftLen + gutter
	}

	words := strings.Fields(text)
	lineLen := 0
	firstWord := true

	for _, word := range words {
		wLen := utf8.RuneCountInString(word)
		needed := wLen
		if !firstWord {
			needed += 1
		}

		if !firstWord && lineLen+needed > descWidth {
			fmt.Fprintf(w, "\n%*s", contIndent, "")
			lineLen = 0
			firstWord = true
		}

		if !firstWord {
			fmt.Fprint(w, " ")
			lineLen++
		}
		fmt.Fprint(w, word)
		lineLen += wLen
		firstWord = false
	}
	fmt.Fprintln(w)
}

// PrintHelp outputs the full help message including usage, options, and positional arguments
func (p *Parser) PrintHelp() {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	prog := filepath.Base(p.programName)
	if prog == "" || prog == "." {
		prog = "program"
	}
	descWidth := 50

	if p.version != "" {
		fmt.Fprintf(p.out, "%s v%s\n\n", prog, p.version)
	}

	fmt.Fprintf(p.out, "Usage: %s [OPTIONS]", prog)
	for _, a := range p.args {
		if a.isPositional && !a.isHidden {
			hint := a.formatTypeHint()
			if a.choices == "" {
				hint = ""
			}
			if a.argType == TypeRest {
				if a.required {
					fmt.Fprintf(p.out, " <%s...%s>", a.name, hint)
				} else {
					fmt.Fprintf(p.out, " [%s...%s]", a.name, hint)
				}
			} else {
				if a.required {
					fmt.Fprintf(p.out, " <%s%s>", a.name, hint)
				} else {
					fmt.Fprintf(p.out, " [%s%s]", a.name, hint)
				}
			}
		}
	}
	fmt.Fprint(p.out, "\n\n")

	maxLen := 20
	for _, a := range p.args {
		if a.isPositional || a.isHidden {
			continue
		}
		longDisp := a.formatLongDisplay()
		length := 2
		if a.shortFlag != "" && longDisp != "" {
			length += utf8.RuneCountInString(a.shortFlag) + 2 + utf8.RuneCountInString(longDisp)
		} else if a.shortFlag != "" {
			length += utf8.RuneCountInString(a.shortFlag)
		} else if longDisp != "" {
			length += 6 + utf8.RuneCountInString(longDisp)
		}
		length += utf8.RuneCountInString(a.formatTypeHint())
		if length > maxLen && length < 42 {
			maxLen = length
		}
	}

	fmt.Fprintln(p.out, "Options:")
	printWrappedDesc(p.out, "  -h, --help", maxLen, 2, "Display this help message", descWidth)

	if p.version != "" {
		var vBuf string
		if p.versionShort != "" {
			vBuf = fmt.Sprintf("  %s, --version", p.versionShort)
		} else {
			vBuf = "      --version"
		}
		printWrappedDesc(p.out, vBuf, maxLen, 2, "Display version information", descWidth)
	}

	for _, a := range p.args {
		if a.isPositional || a.isHidden {
			continue
		}
		longDisp := a.formatLongDisplay()
		var buf string
		if a.shortFlag != "" && longDisp != "" {
			buf = fmt.Sprintf("  %s, %s", a.shortFlag, longDisp)
		} else if a.shortFlag != "" {
			buf = fmt.Sprintf("  %s", a.shortFlag)
		} else if longDisp != "" {
			buf = fmt.Sprintf("      %s", longDisp)
		}
		buf += a.formatTypeHint()

		reqStr := ""
		if a.required {
			reqStr = " [required]"
		}
		defStr := ""
		if a.hasDefault {
			defStr = fmt.Sprintf(" [default: %s]", formatDefaultDisplay(a.defaultVal))
		}
		envStr := ""
		effEnv := p.getEffectiveEnv(a)
		if effEnv != "" {
			envStr = fmt.Sprintf(" [env: %s]", effEnv)
		}

		fullDesc := fmt.Sprintf("%s%s%s%s", a.help, reqStr, defStr, envStr)
		printWrappedDesc(p.out, buf, maxLen, 2, fullDesc, descWidth)

		if len(a.choiceHelps) > 0 {
			for _, ch := range a.choiceHelps {
				prefix := fmt.Sprintf("%*s  %s:", maxLen, "", ch.choice)
				choiceIndent := utf8.RuneCountInString(prefix)
				helpWidth := descWidth - 6
				if helpWidth < 30 {
					helpWidth = 30
				}
				printWrappedDesc(p.out, prefix, choiceIndent, 1, ch.help, helpWidth)
			}
		}
	}

	hasPos := false
	for _, a := range p.args {
		if a.isPositional && !a.isHidden {
			hasPos = true
			break
		}
	}

	if hasPos {
		fmt.Fprintln(p.out, "\nPositional Arguments:")
		for _, a := range p.args {
			if !a.isPositional || a.isHidden {
				continue
			}
			name := a.displayName()
			hint := a.formatTypeHint()
			if a.choices == "" {
				hint = ""
			}
			var buf string
			if a.argType == TypeRest {
				buf = fmt.Sprintf("  <%s...%s>", name, hint)
			} else {
				buf = fmt.Sprintf("  <%s%s>", name, hint)
			}
			reqStr := ""
			if a.required {
				reqStr = " [required]"
			}
			defStr := ""
			if a.hasDefault {
				defStr = fmt.Sprintf(" [default: %s]", formatDefaultDisplay(a.defaultVal))
			}
			effEnv := p.getEffectiveEnv(a)
			envStr := ""
			if effEnv != "" {
				envStr = fmt.Sprintf(" [env: %s]", effEnv)
			}

			fullDesc := fmt.Sprintf("%s%s%s%s", a.help, reqStr, defStr, envStr)
			printWrappedDesc(p.out, buf, maxLen, 2, fullDesc, descWidth)

			if len(a.choiceHelps) > 0 {
				for _, ch := range a.choiceHelps {
					prefix := fmt.Sprintf("%*s  %s:", maxLen, "", ch.choice)
					choiceIndent := utf8.RuneCountInString(prefix)
					helpWidth := descWidth - 6
					if helpWidth < 30 {
						helpWidth = 30
					}
					printWrappedDesc(p.out, prefix, choiceIndent, 1, ch.help, helpWidth)
				}
			}
		}
	}
}

// findFlag retrieves a registered non-positional argument matching the flag string
func (p *Parser) findFlag(flag string, isShort bool) *cliArg {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	for _, a := range p.args {
		if a.isPositional {
			continue
		}
		if isShort && a.shortFlag == flag {
			return a
		}
		if !isShort && a.longFlag == flag {
			return a
		}
	}
	return nil
}

// assignVal parses a string literal, validates it, and only then assigns it to the argument's destination pointer
func (p *Parser) assignVal(arg *cliArg, valStr, flagName string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}

	var (
		boolVal  bool
		charVal  byte
		intVal   int
		floatVal float64
	)

	switch arg.argType {
	case TypeBool:
		switch strings.ToLower(valStr) {
		case "1", "true", "yes", "on":
			boolVal = true
		case "0", "false", "no", "off":
			boolVal = false
		default:
			return fmt.Errorf("%w: invalid boolean value '%s' for flag '%s'", ErrInvalidValue, valStr, flagName)
		}
	case TypeChar:
		if len(valStr) != 1 {
			return fmt.Errorf("%w: invalid character value '%s' for flag '%s' (expected exactly 1 ASCII byte)", ErrInvalidValue, valStr, flagName)
		}
		charVal = valStr[0]
	case TypeInt, TypeCount:
		v, err := strconv.Atoi(valStr)
		if err != nil {
			return fmt.Errorf("%w: invalid integer value '%s' for flag '%s'", ErrInvalidValue, valStr, flagName)
		}
		intVal = v
	case TypeFloat:
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return fmt.Errorf("%w: invalid floating-point value '%s' for flag '%s'", ErrInvalidValue, valStr, flagName)
		}
		floatVal = v
	}

	if !arg.checkChoice(valStr) {
		label := "flag"
		if arg.isPositional {
			label = "positional argument"
		}
		return fmt.Errorf("%w: invalid choice '%s' for %s '%s' (allowed: [%s])",
			ErrInvalidValue, valStr, label, flagName, arg.choices)
	}

	switch arg.argType {
	case TypeBool:
		*(arg.target.(*bool)) = boolVal
	case TypeRest, TypeStrList:
		ptr := arg.target.(*[]string)
		if !arg.isSet && arg.hasDefault {
			*ptr = nil
		}
		*ptr = append(*ptr, valStr)
	case TypeChar:
		*(arg.target.(*byte)) = charVal
	case TypeStr:
		*(arg.target.(*string)) = valStr
	case TypeInt, TypeCount:
		*(arg.target.(*int)) = intVal
	case TypeFloat:
		*(arg.target.(*float64)) = floatVal
	}
	arg.isSet = true
	return nil
}

// isNegativeNumber checks if a token represents a negative numeric literal rather than a flag
func isNegativeNumber(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	if s[1] >= '0' && s[1] <= '9' {
		return true
	}
	return s[1] == '.' && len(s) > 2 && s[2] >= '0' && s[2] <= '9'
}

// Parse processes command-line arguments, validates rules, checks environment variables, and populates bound variables
func (p *Parser) Parse(args []string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if p.initErr != nil {
		return p.initErr
	}

	for _, a := range p.args {
		a.isSet = false
		if (a.argType == TypeRest || a.argType == TypeStrList) && a.target != nil {
			*(a.target.(*[]string)) = nil
		}
		if a.argType == TypeCount && a.target != nil {
			*(a.target.(*int)) = 0
		}
		if a.hasDefault {
			if err := p.assignVal(a, a.defaultVal, a.displayName()); err != nil {
				return err
			}
			a.isSet = false
		}
	}

	for _, rule := range p.rules {
		if rule.ruleType == ruleExclusive {
			tokens := splitTokens(rule.target)
			for _, tok := range tokens {
				if p.findArg(tok) == nil {
					return fmt.Errorf("%w: unknown flag '%s' defined in exclusive rule [%s]", ErrUnknownFlag, tok, rule.target)
				}
			}
		} else if rule.ruleType == ruleDependency {
			if p.findArg(rule.target) == nil {
				return fmt.Errorf("%w: target flag '%s' in dependency rule does not exist", ErrUnknownFlag, rule.target)
			}
			tokens := splitTokens(rule.requires)
			for _, tok := range tokens {
				if p.findArg(tok) == nil {
					return fmt.Errorf("%w: required flag '%s' in dependency rule does not exist", ErrUnknownFlag, tok)
				}
			}
		}
	}

	scanEnd := len(args)
	for i, token := range args {
		if token == "--" {
			scanEnd = i
			break
		}
	}
	scanArgs := args[:scanEnd]

	for i, token := range scanArgs {
		if token == "-h" || token == "--help" {
			var inspect string
			if i+1 < len(scanArgs) && !strings.HasPrefix(scanArgs[i+1], "-") {
				inspect = scanArgs[i+1]
			}

			if inspect != "" {
				isVer := p.version != "" && (inspect == "version" || inspect == "--version" ||
					(p.versionShort != "" && (inspect == p.versionShort || inspect == strings.TrimPrefix(p.versionShort, "-"))))
				if isVer {
					sh := "None"
					if p.versionShort != "" {
						sh = p.versionShort
					}

					fmt.Fprintf(p.out, "%-23s%s\n", "Short flag:", sh)
					fmt.Fprintf(p.out, "%-23s%s\n", "Long flag:", "--version")
					fmt.Fprintf(p.out, "%-23s%v\n", "Value required:", false)
					fmt.Fprintf(p.out, "%-23s%v\n", "Mandatory:", false)
					fmt.Fprintf(p.out, "\nDescription:\n")
					printWrappedDesc(p.out, "", 2, 0, "Display version information", 50)
					return ErrHelpRequested
				}
				if found := p.findArg(inspect); found != nil && !found.isHidden {
					p.printSingleHelp(found)
				} else {
					fmt.Fprintf(p.out, "Argument '%s' not recognized.\n\n", inspect)
					p.PrintHelp()
				}
			} else {
				p.PrintHelp()
			}
			return ErrHelpRequested
		}
	}

	if p.version != "" {
		for _, token := range scanArgs {
			isVer := (token == "--version")
			if !isVer && p.versionShort != "" && token == p.versionShort {
				isVer = true
			}
			if isVer {
				prog := filepath.Base(p.programName)
				if prog == "" || prog == "." {
					prog = "program"
				}
				fmt.Fprintf(p.out, "%s %s\n", prog, p.version)
				if p.versionExtra != "" {
					fmt.Fprintln(p.out, p.versionExtra)
				}
				return ErrVersionRequested
			}
		}
	}

	stopFlags := false
	currentPosIndex := 0

	for i := 0; i < len(args); i++ {
		token := args[i]

		if !stopFlags && token == "--" {
			stopFlags = true
			continue
		}

		if stopFlags || !strings.HasPrefix(token, "-") || token == "-" {
			var targetPos *cliArg
			foundPos := 0
			for _, a := range p.args {
				if a.isPositional {
					if foundPos == currentPosIndex {
						targetPos = a
						break
					}
					foundPos++
				}
			}

			if targetPos != nil {
				if err := p.assignVal(targetPos, token, targetPos.displayName()); err != nil {
					return err
				}
				if targetPos.argType != TypeRest {
					currentPosIndex++
				}
			} else {
				return fmt.Errorf("%w: unexpected positional argument '%s'", ErrUnknownFlag, token)
			}
			continue
		}

		if strings.HasPrefix(token, "--") {
			var flagName, inlineVal string
			hasInline := false

			if idx := strings.IndexByte(token, '='); idx != -1 {
				flagName = token[:idx]
				inlineVal = token[idx+1:]
				hasInline = true
			} else {
				flagName = token
			}

			matched := p.findFlag(flagName, false)
			isNegated := false
			if matched == nil && strings.HasPrefix(flagName, "--no-") {
				positiveFlag := "--" + strings.TrimPrefix(flagName, "--no-")
				if alt := p.findFlag(positiveFlag, false); alt != nil && alt.argType == TypeBool {
					matched = alt
					isNegated = true
				}
			}

			if matched == nil {
				return fmt.Errorf("%w: unknown flag '%s'", ErrUnknownFlag, flagName)
			}

			if matched.argType == TypeBool {
				val := true
				if isNegated {
					val = false
				} else if hasInline {
					lower := strings.ToLower(inlineVal)
					switch lower {
					case "true", "1", "yes", "on":
						val = true
					case "false", "0", "no", "off":
						val = false
					default:
						return fmt.Errorf("%w: invalid boolean value '%s' for flag '%s'", ErrInvalidValue, inlineVal, flagName)
					}
				}
				*(matched.target.(*bool)) = val
				matched.isSet = true
				continue
			}

			if matched.argType == TypeCount {
				*(matched.target.(*int))++
				matched.isSet = true
				continue
			}

			valStr := inlineVal
			if !hasInline {
				if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-" && !isNegativeNumber(args[i+1])) {
					return fmt.Errorf("%w: flag '%s' requires a value", ErrMissingValue, flagName)
				}
				i++
				valStr = args[i]
			}

			if err := p.assignVal(matched, valStr, flagName); err != nil {
				return err
			}
			continue
		}

		tokenRunes := []rune(token)
		for c := 1; c < len(tokenRunes); c++ {
			singleSh := "-" + string(tokenRunes[c])
			matched := p.findFlag(singleSh, true)

			if matched == nil {
				return fmt.Errorf("%w: unknown short flag '%s' in '%s'", ErrUnknownFlag, singleSh, token)
			}

			if matched.argType == TypeBool {
				*(matched.target.(*bool)) = true
				matched.isSet = true
			} else if matched.argType == TypeCount {
				*(matched.target.(*int))++
				matched.isSet = true
			} else {
				var valStr string
				if c+1 < len(tokenRunes) {
					valStr = string(tokenRunes[c+1:])
					if strings.HasPrefix(valStr, "=") {
						valStr = strings.TrimPrefix(valStr, "=")
					}
					c = len(tokenRunes)
				} else {
					if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-" && !isNegativeNumber(args[i+1])) {
						return fmt.Errorf("%w: flag '%s' requires a value", ErrMissingValue, singleSh)
					}
					i++
					valStr = args[i]
				}
				if err := p.assignVal(matched, valStr, singleSh); err != nil {
					return err
				}
				break
			}
		}
	}

	for _, arg := range p.args {
		effEnv := p.getEffectiveEnv(arg)
		if !arg.isSet && effEnv != "" {
			if envVal, ok := os.LookupEnv(effEnv); ok && envVal != "" {
				if err := p.assignVal(arg, envVal, effEnv); err != nil {
					return err
				}
			}
		}
	}

	for _, rule := range p.rules {
		if rule.ruleType == ruleExclusive {
			tokens := splitTokens(rule.target)
			var setFlags []string
			for _, tok := range tokens {
				if a := p.findArg(tok); a != nil && a.isSet {
					setFlags = append(setFlags, a.displayName())
				}
			}
			if len(setFlags) > 1 {
				return fmt.Errorf("%w: mutually exclusive flags in group [%s] cannot be used together (conflicting: '%s' and '%s')",
					ErrMutualExclusion, rule.target, setFlags[0], setFlags[1])
			}
		} else if rule.ruleType == ruleDependency {
			target := p.findArg(rule.target)
			if target != nil && target.isSet {
				reqTokens := splitTokens(rule.requires)
				for _, req := range reqTokens {
					dep := p.findArg(req)
					if dep == nil || !dep.isSet {
						return fmt.Errorf("%w: flag '%s' requires '%s' to be set",
							ErrDependency, target.displayName(), req)
					}
				}
			}
		}
	}

	for _, a := range p.args {
		if a.required && !a.isSet {
			if a.isPositional {
				return fmt.Errorf("%w: mandatory positional argument missing: <%s>", ErrMissingRequired, a.name)
			}
			return fmt.Errorf("%w: mandatory flag missing: %s", ErrMissingRequired, a.displayName())
		}
	}

	return nil
}

// SetVersion configures version details and enables the --version flag along with an optional short alias
func (p *Parser) SetVersion(version, sh, extraInfo string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.version = version
	p.versionExtra = extraInfo
	if strings.TrimSpace(sh) != "" {
		p.versionShort = normalizeShort(sh)
	} else {
		p.versionShort = ""
	}
}
