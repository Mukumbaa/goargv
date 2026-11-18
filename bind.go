package goargv

import (
	"fmt"
	"strings"
)

// addArg registers an argument definition and checks that its target pointer and flags are valid
func (p *Parser) addArg(name, sh, lng string, t CliType, target any, required, isPos bool, choices, help string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	isNilTarget := target == nil
	if !isNilTarget {
		switch v := target.(type) {
		case *bool:
			isNilTarget = (v == nil)
		case *byte:
			isNilTarget = (v == nil)
		case *int:
			isNilTarget = (v == nil)
		case *float64:
			isNilTarget = (v == nil)
		case *string:
			isNilTarget = (v == nil)
		case *[]string:
			isNilTarget = (v == nil)
		case *[]int:
			isNilTarget = (v == nil)
		case *[]float64:
			isNilTarget = (v == nil)
		}
	}
	if isNilTarget {
		displayName := lng
		if displayName == "" {
			displayName = sh
		}
		if displayName == "" {
			displayName = name
		}
		panic(fmt.Sprintf("goargv: target pointer for argument '%s' cannot be nil", displayName))
	}

	normSh := normalizeShort(sh)
	normLng := normalizeLong(lng)

	if isPos {
		for _, a := range p.args {
			if a.isPositional && a.argType == TypeRest {
				panic(fmt.Sprintf("goargv: cannot bind positional argument '%s' after a rest argument", name))
			}
			if a.isPositional && name != "" && a.name == name {
				panic(fmt.Sprintf("goargv: duplicate positional argument '%s'", name))
			}
		}
	} else {
		if normSh == "-h" {
			panic("goargv: flag '-h' conflicts with reserved help flag '-h'")
		}
		if normLng == "--help" {
			panic("goargv: flag '--help' conflicts with reserved help flag '--help'")
		}
		if p.version != "" {
			if p.versionShort != "" && normSh == p.versionShort {
				panic(fmt.Sprintf("goargv: flag '%s' conflicts with reserved version short flag '%s'", normSh, p.versionShort))
			}
			if normLng == "--version" {
				panic("goargv: flag '--version' conflicts with reserved version flag '--version'")
			}
		}
		for _, a := range p.args {
			if a.isPositional {
				continue
			}
			if normSh != "" && a.shortFlag == normSh {
				panic(fmt.Sprintf("goargv: duplicate short flag '%s'", normSh))
			}
			if normLng != "" && a.longFlag == normLng {
				panic(fmt.Sprintf("goargv: duplicate long flag '%s'", normLng))
			}
		}
	}

	var defBool bool
	if t == TypeBool {
		defBool = *(target.(*bool))
	}
	p.args = append(p.args, &cliArg{
		name:         name,
		shortFlag:    normSh,
		longFlag:     normLng,
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

// BindIntList binds a slice to collect repeated integer occurrences of a flag (e.g. -p 80 -p 443)
func (p *Parser) BindIntList(sh, lng string, target *[]int, required bool, choices, help string) {
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
	p.addArg("", sh, lng, TypeIntList, target, required, false, choices, help)
}

// BindFloatList binds a slice to collect repeated float64 occurrences of a flag
func (p *Parser) BindFloatList(sh, lng string, target *[]float64, required bool, choices, help string) {
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
	p.addArg("", sh, lng, TypeFloatList, target, required, false, choices, help)
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

// SetPlaceholder sets a custom type placeholder label shown in help output (e.g. "path" -> <path>)
func (p *Parser) SetPlaceholder(flagOrName, placeholder string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found to set placeholder", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	arg.placeholder = strings.TrimSpace(placeholder)
	return nil
}

// SetHelpGroup assigns an option to a named section in help output
func (p *Parser) SetHelpGroup(flagOrName, group string) error {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	arg := p.findArg(flagOrName)
	if arg == nil {
		err := fmt.Errorf("%w: argument '%s' not found to set help group", ErrUnknownFlag, flagOrName)
		if p.initErr == nil {
			p.initErr = err
		}
		return err
	}
	arg.helpGroup = strings.TrimSpace(group)
	return nil
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
