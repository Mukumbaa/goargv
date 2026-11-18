package goargv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (p *Parser) getDescWidth(leftColWidth int) int {
	if p.helpWidth > 0 {
		return p.helpWidth
	}
	if colsStr := os.Getenv("COLUMNS"); colsStr != "" {
		if cols, err := strconv.Atoi(strings.TrimSpace(colsStr)); err == nil && cols > 0 {
			avail := cols - leftColWidth
			if avail < 30 {
				avail = 30
			}
			return avail
		}
	}
	return 50
}

func (p *Parser) printOptionHelp(a *cliArg, maxLen, descWidth int) {
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

func (a *cliArg) formatPositionalLabel() string {
	if a.placeholder != "" {
		ph := strings.TrimSuffix(strings.TrimPrefix(a.placeholder, "<"), ">")
		if a.argType == TypeRest {
			return ph + "..."
		}
		return ph
	}
	hint := ""
	if a.choices != "" && len(a.choiceHelps) < a.countChoices() {
		hint = fmt.Sprintf(" {%s}", a.choices)
	}
	if a.argType == TypeRest {
		return a.name + "..." + hint
	}
	return a.displayName() + hint
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

	if p.version != "" {
		fmt.Fprintf(p.out, "%s v%s\n\n", prog, p.version)
	}

	if p.description != "" {
		fmt.Fprintf(p.out, "%s\n\n", p.description)
	}

	fmt.Fprintf(p.out, "Usage: %s [OPTIONS]", prog)
	for _, a := range p.args {
		if a.isPositional && !a.isHidden {
			label := a.formatPositionalLabel()
			if a.required {
				fmt.Fprintf(p.out, " <%s>", label)
			} else {
				fmt.Fprintf(p.out, " [%s]", label)
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
			length += 4 + utf8.RuneCountInString(longDisp)
		}
		length += utf8.RuneCountInString(a.formatTypeHint())
		if length > maxLen && length < 42 {
			maxLen = length
		}
	}

	descWidth := p.getDescWidth(maxLen + 2)

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

	var groups []string
	seenGroup := make(map[string]bool)

	for _, a := range p.args {
		if a.isPositional || a.isHidden {
			continue
		}
		if a.helpGroup == "" {
			p.printOptionHelp(a, maxLen, descWidth)
		} else if !seenGroup[a.helpGroup] {
			seenGroup[a.helpGroup] = true
			groups = append(groups, a.helpGroup)
		}
	}

	for _, grp := range groups {
		fmt.Fprintf(p.out, "\n%s:\n", strings.TrimSuffix(grp, ":"))
		for _, a := range p.args {
			if !a.isPositional && !a.isHidden && a.helpGroup == grp {
				p.printOptionHelp(a, maxLen, descWidth)
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
			buf := fmt.Sprintf("  <%s>", a.formatPositionalLabel())
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

	if p.epilog != "" {
		fmt.Fprintf(p.out, "\n%s\n", p.epilog)
	}
}

// printSingleHelp prints a detailed view of a single argument's configuration and choices
func (p *Parser) printSingleHelp(arg *cliArg) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	descWidth := p.getDescWidth(23)

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

	if text == "" {
		if leftLen > 0 {
			fmt.Fprintln(w, leftCol)
		} else {
			fmt.Fprintln(w)
		}
		return
	}

	if leftLen > 0 {
		fmt.Fprint(w, leftCol)
		if leftLen <= indent {
			fmt.Fprintf(w, "%*s", indent-leftLen+gutter, "")
		} else {
			fmt.Fprintf(w, "\n%*s", indent+gutter, "")
		}
	} else {
		fmt.Fprintf(w, "%*s", indent+gutter, "")
	}

	contIndent := indent + gutter

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

// formatTypeHint renders the CLI usage placeholder or choice summary for the argument
func (arg *cliArg) formatTypeHint() string {
	if arg.placeholder != "" {
		ph := arg.placeholder
		if !strings.HasPrefix(ph, "<") && !strings.HasPrefix(ph, "{") && !strings.HasPrefix(ph, "[") {
			ph = "<" + ph + ">"
		}
		if arg.argType == TypeStrList || arg.argType == TypeIntList || arg.argType == TypeFloatList {
			return " " + ph + "..."
		}
		return " " + ph
	}
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
	case TypeIntList:
		return " <int>..."
	case TypeFloat:
		return " <num>"
	case TypeFloatList:
		return " <num>..."
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

