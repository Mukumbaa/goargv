package goargv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrHelpRequested   = errors.New("help requested")
	ErrUnknownFlag     = errors.New("unknown flag")
	ErrMissingValue    = errors.New("missing value")
	ErrInvalidValue    = errors.New("invalid value")
	ErrMissingRequired = errors.New("missing required argument")
	ErrMutualExclusion = errors.New("mutually exclusive flags")
	ErrDependency      = errors.New("flag dependency not satisfied")
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
	argType      CliType
	target       any
	isPositional bool
	required     bool
	isSet        bool
}

type Parser struct {
	programName string
	args        []*cliArg
	rules       []cliRule
	initErr     error 
}

func New(programName string) *Parser {
	return &Parser{
		programName: programName,
		args:        make([]*cliArg, 0, 8),
		rules:       make([]cliRule, 0),
	}
}

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

func (p *Parser) addArg(name, sh, lng string, t CliType, target any, required, isPos bool, choices, help string) {
	p.args = append(p.args, &cliArg{
		name:         name,
		shortFlag:    normalizeShort(sh),
		longFlag:     normalizeLong(lng),
		argType:      t,
		target:       target,
		required:     required,
		isPositional: isPos,
		choices:      choices,
		choiceHelps:  make([]cliChoiceHelp, 0),
		help:         help,
	})
}

func (p *Parser) BindBool(sh, lng string, target *bool, help string) {
	p.addArg("", sh, lng, TypeBool, target, false, false, "", help)
}

func (p *Parser) BindCount(sh, lng string, target *int, help string) {
	p.addArg("", sh, lng, TypeCount, target, false, false, "", help)
}

func (p *Parser) BindChar(sh, lng string, target *byte, required bool, choices, help string) {
	p.addArg("", sh, lng, TypeChar, target, required, false, choices, help)
}

func (p *Parser) BindStr(sh, lng string, target *string, required bool, choices, help string) {
	p.addArg("", sh, lng, TypeStr, target, required, false, choices, help)
}

func (p *Parser) BindInt(sh, lng string, target *int, required bool, choices, help string) {
	p.addArg("", sh, lng, TypeInt, target, required, false, choices, help)
}

func (p *Parser) BindFloat(sh, lng string, target *float64, required bool, choices, help string) {
	p.addArg("", sh, lng, TypeFloat, target, required, false, choices, help)
}

func (p *Parser) BindPos(name string, target *string, required bool, choices, help string) {
	p.addArg(name, "", "", TypeStr, target, required, true, choices, help)
}

func (p *Parser) BindRest(name string, target *[]string, required bool, choices, help string) {
	*target = nil
	p.addArg(name, "", "", TypeRest, target, required, true, choices, help)
}

func (p *Parser) findArg(ident string) *cliArg {
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

func (p *Parser) SetEnv(flagOrName, envVar string) {
	if a := p.findArg(flagOrName); a != nil {
		a.envVar = envVar
	}
}

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

func (p *Parser) SetChoiceHelp(flagOrName, choice, help string) error {
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

func (p *Parser) AddExclusive(flagsGroup string) {
	p.rules = append(p.rules, cliRule{
		ruleType: ruleExclusive,
		target:   flagsGroup,
	})
}

func (p *Parser) AddDependency(flag, requiresFlags string) {
	p.rules = append(p.rules, cliRule{
		ruleType: ruleDependency,
		target:   flag,
		requires: requiresFlags,
	})
}

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

func (arg *cliArg) checkChoice(valStr string) bool {
	if arg.choices == "" {
		return true
	}
	if arg.argType == TypeChar && !strings.ContainsAny(arg.choices, ",|") {
		return strings.Contains(arg.choices, valStr)
	}

	tokens := splitTokens(arg.choices)
	for _, token := range tokens {
		switch arg.argType {
		case TypeStr, TypeChar, TypeRest:
			if token == valStr {
				return true
			}
		case TypeInt:
			if v, err := strconv.Atoi(token); err == nil && v == *(arg.target.(*int)) {
				return true
			}
		case TypeFloat:
			if v, err := strconv.ParseFloat(token, 64); err == nil && v == *(arg.target.(*float64)) {
				return true
			}
		}
	}
	return false
}

func (arg *cliArg) formatTypeHint() string {
	if arg.choices != "" {
		return fmt.Sprintf(" {%s}", arg.choices)
	}
	switch arg.argType {
	case TypeStr:
		return " <str>"
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

func (p *Parser) PrintSingleHelp(arg *cliArg) {
	descWidth := 50

	if arg.isPositional {
		fmt.Printf("%-23s<%s>\n", "Positional argument:", arg.displayName())
		fmt.Printf("%-23s%v\n", "Mandatory:", arg.required)
	} else {
		sh := "None"
		if arg.shortFlag != "" {
			sh = arg.shortFlag
		}
		lng := "None"
		if arg.longFlag != "" {
			lng = arg.longFlag
		}
		fmt.Printf("%-23s%s\n", "Short flag:", sh)
		fmt.Printf("%-23s%s\n", "Long flag:", lng)
		valReq := arg.argType != TypeBool && arg.argType != TypeCount
		fmt.Printf("%-23s%v\n", "Value required:", valReq)
		fmt.Printf("%-23s%v\n", "Mandatory:", arg.required)
	}
	if arg.envVar != "" {
		fmt.Printf("%-23s%s\n", "Environment variable:", arg.envVar)
	}
	if arg.choices != "" {
		fmt.Printf("Allowed choices:\n")
		if len(arg.choiceHelps) > 0 {
			maxCLen := 0
			for _, ch := range arg.choiceHelps {
				if len(ch.choice) > maxCLen {
					maxCLen = len(ch.choice)
				}
			}
			for _, ch := range arg.choiceHelps {
				prefix := fmt.Sprintf("  %-*s:", maxCLen, ch.choice)
				printWrappedDesc(prefix, len(prefix),1, ch.help, descWidth)
			}
		}
	}
	fmt.Printf("\nDescription:\n")
	help := "No documentation available."
	if arg.help != "" {
		help = arg.help
	}
	printWrappedDesc("", 2,0, help, descWidth)
}

func printWrappedDesc(leftCol string, indent int, gutter int, text string, descWidth int) {
	leftLen := len(leftCol)

	if leftLen > 0 {
		fmt.Print(leftCol)
		if leftLen < indent {
			fmt.Printf("%*s", indent-leftLen+gutter, "")
		} else {
			fmt.Printf("%*s", gutter, "")
		}
	} else {
		fmt.Printf("%*s", indent+gutter, "")
	}

	if text == "" {
		fmt.Println()
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
		needed := len(word)
		if !firstWord {
			needed += 1
		}

		if !firstWord && lineLen+needed > descWidth {
			fmt.Printf("\n%*s", contIndent, "")
			lineLen = 0
			firstWord = true
		}

		if !firstWord {
			fmt.Print(" ")
			lineLen++
		}
		fmt.Print(word)
		lineLen += len(word)
		firstWord = false
	}
	fmt.Println()
}

func (p *Parser) PrintHelp() {
	prog := filepath.Base(p.programName)
	if prog == "" || prog == "." {
		prog = "program"
	}

	descWidth := 50

	fmt.Printf("Usage: %s [OPTIONS]", prog)

	for _, a := range p.args {
		if a.isPositional {
			hint := a.formatTypeHint()
			if a.choices == "" {
				hint = ""
			}
			if a.argType == TypeRest {
				if a.required {
					fmt.Printf(" <%s...%s>", a.name, hint)
				} else {
					fmt.Printf(" [%s...%s]", a.name, hint)
				}
			} else {
				if a.required {
					fmt.Printf(" <%s%s>", a.name, hint)
				} else {
					fmt.Printf(" [%s%s]", a.name, hint)
				}
			}
		}
	}
	fmt.Print("\n\n")

	maxLen := 20
	for _, a := range p.args {
		if a.isPositional {
			continue
		}
		length := 2
		if a.shortFlag != "" && a.longFlag != "" {
			length += len(a.shortFlag) + 2 + len(a.longFlag)
		} else if a.shortFlag != "" {
			length += len(a.shortFlag)
		} else if a.longFlag != "" {
			length += 6 + len(a.longFlag)
		}
		length += len(a.formatTypeHint())
		if length > maxLen && length < 42 {
			maxLen = length
		}
	}

	fmt.Println("Options:")
	printWrappedDesc("  -h, --help", maxLen,2, "Display this help message", descWidth)

	for _, a := range p.args {
		if a.isPositional {
			continue
		}
		var buf string
		if a.shortFlag != "" && a.longFlag != "" {
			buf = fmt.Sprintf("  %s, %s", a.shortFlag, a.longFlag)
		} else if a.shortFlag != "" {
			buf = fmt.Sprintf("  %s", a.shortFlag)
		} else if a.longFlag != "" {
			buf = fmt.Sprintf("      %s", a.longFlag)
		}
		buf += a.formatTypeHint()

		reqStr := ""
		if a.required {
			reqStr = " [required]"
		}
		envStr := ""
		if a.envVar != "" {
			envStr = fmt.Sprintf(" [env: %s]", a.envVar)
		}

		fullDesc := fmt.Sprintf("%s%s%s", a.help, reqStr, envStr)
		printWrappedDesc(buf, maxLen,2, fullDesc, descWidth)

		if len(a.choiceHelps) > 0 {
			for _, ch := range a.choiceHelps {
				prefix := fmt.Sprintf("%*s  %s:", maxLen, "", ch.choice)
				choiceIndent := len(prefix)
				helpWidth := descWidth - 6
				if helpWidth < 30 {
					helpWidth = 30
				}
				printWrappedDesc(prefix, choiceIndent, 1, ch.help, helpWidth)
			}
		}
	}

	hasPos := false
	for _, a := range p.args {
		if a.isPositional {
			hasPos = true
			break
		}
	}

	if hasPos {
		fmt.Println("\nPositional Arguments:")
		for _, a := range p.args {
			if !a.isPositional {
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
			envStr := ""
			if a.envVar != "" {
				envStr = fmt.Sprintf(" [env: %s]", a.envVar)
			}

			fullDesc := fmt.Sprintf("%s%s%s", a.help, reqStr, envStr)
			printWrappedDesc(buf, maxLen,2, fullDesc, descWidth)

			if len(a.choiceHelps) > 0 {
				for _, ch := range a.choiceHelps {
					prefix := fmt.Sprintf("%*s  %s:", maxLen, "", ch.choice)
					choiceIndent := len(prefix)
					helpWidth := descWidth - 6
					if helpWidth < 30 {
						helpWidth = 30
					}
					printWrappedDesc(prefix, choiceIndent,1, ch.help, helpWidth)
				}
			}
		}
	}
}

func (p *Parser) findFlag(flag string, isShort bool) *cliArg {
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

func (p *Parser) assignVal(arg *cliArg, valStr, flagName string) error {
	arg.isSet = true

	switch arg.argType {
	case TypeRest:
		ptr := arg.target.(*[]string)
		*ptr = append(*ptr, valStr)
	case TypeChar:
		if len(valStr) != 1 {
			return fmt.Errorf("%w: invalid character value '%s' for flag '%s' (expected 1 char)", ErrInvalidValue, valStr, flagName)
		}
		*(arg.target.(*byte)) = valStr[0]
	case TypeStr:
		*(arg.target.(*string)) = valStr
	case TypeInt:
		v, err := strconv.Atoi(valStr)
		if err != nil {
			return fmt.Errorf("%w: invalid integer value '%s' for flag '%s'", ErrInvalidValue, valStr, flagName)
		}
		*(arg.target.(*int)) = v
	case TypeFloat:
		v, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return fmt.Errorf("%w: invalid floating-point value '%s' for flag '%s'", ErrInvalidValue, valStr, flagName)
		}
		*(arg.target.(*float64)) = v
	}

	if !arg.checkChoice(valStr) {
		label := "flag"
		if arg.isPositional {
			label = "positional argument"
		}
		return fmt.Errorf("%w: invalid choice '%s' for %s '%s' (allowed: [%s])",
			ErrInvalidValue, valStr, label, flagName, arg.choices)
	}

	return nil
}

func isNegativeNumber(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	return (s[1] >= '0' && s[1] <= '9') || s[1] == '.'
}

func (p *Parser) Parse(args []string) error {
	if p.initErr != nil {
		return p.initErr
	}

	for i, token := range args {
		if token == "-h" || token == "--help" {
			var inspect string
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				inspect = args[i+1]
			} else if i > 0 && !strings.HasPrefix(args[i-1], "-") {
				inspect = args[i-1]
			}

			if inspect != "" {
				if found := p.findArg(inspect); found != nil {
					p.PrintSingleHelp(found)
				} else {
					fmt.Printf("Argument '%s' not recognized.\n\n", inspect)
					p.PrintHelp()
				}
			} else {
				p.PrintHelp()
			}
			return ErrHelpRequested
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
			if matched == nil {
				return fmt.Errorf("%w: unknown flag '%s'", ErrUnknownFlag, flagName)
			}

			if matched.argType == TypeBool {
				*(matched.target.(*bool)) = true
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
				if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && !isNegativeNumber(args[i+1])) {
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
					c = len(tokenRunes)
				} else {
					if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && !isNegativeNumber(args[i+1])) {
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
		if !arg.isSet && arg.envVar != "" {
			if envVal, ok := os.LookupEnv(arg.envVar); ok && envVal != "" {
				if arg.argType == TypeBool {
					lower := strings.ToLower(envVal)
					if lower == "1" || lower == "true" || lower == "yes" || lower == "on" {
						*(arg.target.(*bool)) = true
						arg.isSet = true
					}
				} else if arg.argType == TypeCount {
					if c, err := strconv.Atoi(envVal); err == nil {
						*(arg.target.(*int)) = c
						arg.isSet = true
					}
				} else {
					if err := p.assignVal(arg, envVal, arg.envVar); err != nil {
						return err
					}
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
