package goargv

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
			if i+1 < len(scanArgs) && scanArgs[i+1] != "-h" && scanArgs[i+1] != "--help" {
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
