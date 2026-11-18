package goargv

import (
	"errors"
	"io"
	"os"
	"strings"
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
	TypeIntList
	TypeFloatList
)

type ruleType int

const (
	ruleExclusive ruleType = iota
	ruleDependency
	ruleAnyOf
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
	placeholder  string
	helpGroup    string
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
	programName      string
	version          string
	versionShort     string
	versionExtra     string
	description      string
	epilog           string
	envPrefix        string
	helpWidth        int
	stopAtPositional bool
	remainingArgs    []string
	out              io.Writer
	args             []*cliArg
	rules            []cliRule
	initErr          error
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

// SetDescription sets a summary description of the program displayed in help output
func (p *Parser) SetDescription(desc string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.description = strings.TrimSpace(desc)
}

// SetEpilog sets a trailing text block displayed at the end of help output
func (p *Parser) SetEpilog(epilog string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.epilog = strings.TrimSpace(epilog)
}

// SetHelpWidth configures the description wrapping width in characters (0 = auto from COLUMNS or default 50)
func (p *Parser) SetHelpWidth(width int) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	if width < 0 {
		width = 0
	}
	p.helpWidth = width
}

// SetVersion configures version details and enables the --version flag along with an optional short alias
func (p *Parser) SetVersion(version, sh, extraInfo string) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	var normSh string
	if strings.TrimSpace(sh) != "" {
		normSh = normalizeShort(sh)
	}
	if version != "" {
		if normSh == "-h" {
			panic("goargv: version short flag '-h' conflicts with reserved help flag '-h'")
		}
		for _, a := range p.args {
			if a.isPositional {
				continue
			}
			if normSh != "" && a.shortFlag == normSh {
				panic("goargv: version short flag '" + normSh + "' conflicts with already bound flag")
			}
			if a.longFlag == "--version" {
				panic("goargv: version flag '--version' conflicts with already bound flag")
			}
		}
	}
	p.version = version
	p.versionExtra = extraInfo
	p.versionShort = normSh
}

// SetStopAtPositional stops flag and help parsing at the first positional argument,
// storing all trailing tokens in RemainingArgs
func (p *Parser) SetStopAtPositional(stop bool) {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	p.stopAtPositional = stop
}

// RemainingArgs returns the unparsed trailing arguments captured after stopping at a positional argument
func (p *Parser) RemainingArgs() []string {
	if p == nil {
		panic("goargv: Parser instance is nil")
	}
	return p.remainingArgs
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
