package goargv

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestBoolNegationAndFormats(t *testing.T) {
	var b1, b2, b3, b4 bool
	b2 = true
	b4 = true

	p := New("app")
	p.BindBool("a", "alpha", &b1, "Alpha")
	p.BindBool("b", "beta", &b2, "Beta")
	p.BindBool("c", "gamma", &b3, "Gamma")
	p.BindBool("d", "delta", &b4, "Delta")

	if err := p.Parse([]string{"-a", "--no-beta", "--gamma=yes", "--delta=0"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !b1 || b2 || !b3 || b4 {
		t.Fatalf("unexpected bools: b1=%v b2=%v b3=%v b4=%v", b1, b2, b3, b4)
	}

	if err := p.Parse([]string{"--alpha=maybe"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
}

func TestShortClusteringAndAttachedValues(t *testing.T) {
	var flagA, flagB bool
	var count, num int
	var str string

	p := New("app")
	p.BindBool("a", "alpha", &flagA, "A")
	p.BindBool("b", "beta", &flagB, "B")
	p.BindCount("v", "verbose", &count, "V")
	p.BindInt("n", "num", &num, false, "", "N")
	p.BindStr("s", "str", &str, false, "", "S")

	if err := p.Parse([]string{"-abvv", "-n42", "-s=hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flagA || !flagB || count != 2 || num != 42 || str != "hello" {
		t.Fatalf("unexpected parsed values: a=%v b=%v count=%d num=%d str=%q", flagA, flagB, count, num, str)
	}

	if err := p.Parse([]string{"-ax"}); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected ErrUnknownFlag, got %v", err)
	}
}

func TestNumbersNegativeDotAndStdin(t *testing.T) {
	var i int
	var f1, f2 float64
	var out string

	p := New("app")
	p.BindInt("i", "int", &i, false, "", "Int")
	p.BindFloat("f", "flt", &f1, false, "", "Float1")
	p.BindFloat("g", "flt2", &f2, false, "", "Float2")
	p.BindStr("o", "out", &out, false, "", "Out")

	if err := p.Parse([]string{"-i", "-12", "-f", "-.5", "--flt2=-3.25", "-o", "-"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i != -12 || f1 != -0.5 || f2 != -3.25 || out != "-" {
		t.Fatalf("unexpected values: i=%d f1=%f f2=%f out=%q", i, f1, f2, out)
	}

	if err := p.Parse([]string{"-i", "not_int"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
}

func TestPositionalRestAndDoubleDash(t *testing.T) {
	var pos string
	var rest []string
	var flag bool

	p := New("app")
	p.BindBool("f", "flag", &flag, "Flag")
	p.BindPos("cmd", &pos, true, "", "Command")
	p.BindRest("files", &rest, false, "", "Files")

	if err := p.Parse([]string{"run", "--", "--flag", "-h", "--version"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag || pos != "run" || len(rest) != 3 || rest[0] != "--flag" || rest[1] != "-h" || rest[2] != "--version" {
		t.Fatalf("unexpected values: flag=%v pos=%q rest=%v", flag, pos, rest)
	}
}

func TestMissingValuesAndUnknownFlags(t *testing.T) {
	var s1, s2 string
	p := New("app")
	p.BindStr("a", "alpha", &s1, false, "", "A")
	p.BindStr("b", "beta", &s2, false, "", "B")

	cases := []struct {
		args []string
		want error
	}{
		{[]string{"--alpha"}, ErrMissingValue},
		{[]string{"--alpha", "--beta", "ok"}, ErrMissingValue},
		{[]string{"--nonexistent"}, ErrUnknownFlag},
		{[]string{"unexpected"}, ErrUnknownFlag},
	}

	for _, tc := range cases {
		if err := p.Parse(tc.args); !errors.Is(err, tc.want) {
			t.Errorf("args=%v: expected %v, got %v", tc.args, tc.want, err)
		}
	}
}

func TestDefaultsEnvPriorityAndReparse(t *testing.T) {
	var port, count int
	var host string
	var inc, rest []string

	p := New("app")
	p.SetEnvPrefix("  TESTAPP  ")
	p.BindInt("p", "port", &port, false, "", "Port")
	p.BindStr("H", "host", &host, false, "", "Host")
	p.BindCount("v", "verbose", &count, "Verbose")
	p.BindStrList("I", "include", &inc, false, "", "Include")
	p.BindRest("rest", &rest, false, "", "Rest")

	_ = p.SetDefault("port", "8080")
	_ = p.SetDefault("host", "127.0.0.1")
	_ = p.SetDefault("verbose", "2")
	_ = p.SetDefault("include", "/def/inc")
	_ = p.SetDefault("rest", "def.txt")

	if err := p.Parse([]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 8080 || host != "127.0.0.1" || count != 2 ||
		len(inc) != 1 || inc[0] != "/def/inc" || len(rest) != 1 || rest[0] != "def.txt" {
		t.Fatalf("unexpected defaults: port=%d host=%q count=%d inc=%v rest=%v", port, host, count, inc, rest)
	}

	t.Setenv("TESTAPP_PORT", "9090")
	t.Setenv("TESTAPP_HOST", "env.local")
	if err := p.Parse([]string{"--port", "3000", "-I", "/cli/inc", "cli.txt"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 3000 || host != "env.local" || count != 2 ||
		len(inc) != 1 || inc[0] != "/cli/inc" || len(rest) != 1 || rest[0] != "cli.txt" {
		t.Fatalf("unexpected values: port=%d host=%q count=%d inc=%v rest=%v", port, host, count, inc, rest)
	}
}

func TestInvalidEnvCountReturnsError(t *testing.T) {
	var count int
	p := New("app")
	p.BindCount("v", "verbose", &count, "Verbose")
	if err := p.SetEnv("verbose", "CUSTOM_VERBOSE"); err != nil {
		t.Fatalf("unexpected SetEnv error: %v", err)
	}

	t.Setenv("CUSTOM_VERBOSE", "not_an_int")
	if err := p.Parse([]string{}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
}

func TestChoicesAndRules(t *testing.T) {
	var mode byte
	var level int
	var ratio float64
	var fmtStr string
	var json, xml, auth bool
	var token, user, input string

	p := New("app")
	p.BindChar("m", "mode", &mode, false, "abc", "Mode")
	p.BindInt("l", "level", &level, false, "1, 2 | 3", "Level")
	p.BindFloat("r", "ratio", &ratio, false, "0.5, 1.5", "Ratio")
	p.BindStr("f", "format", &fmtStr, false, "json|yaml", "Format")
	p.BindBool("j", "json", &json, "JSON")
	p.BindBool("x", "xml", &xml, "XML")
	p.BindBool("a", "auth", &auth, "Auth")
	p.BindStr("t", "token", &token, false, "", "Token")
	p.BindStr("u", "user", &user, false, "", "User")
	p.BindPos("input", &input, true, "", "Input")

	p.AddExclusive("json | xml")
	p.AddDependency("auth", "token, user")

	cases := []struct {
		args []string
		want error
	}{
		{[]string{}, ErrMissingRequired},
		{[]string{"-m", "z", "in.txt"}, ErrInvalidValue},
		{[]string{"-l", "9", "in.txt"}, ErrInvalidValue},
		{[]string{"-r", "2.0", "in.txt"}, ErrInvalidValue},
		{[]string{"-f", "xml", "in.txt"}, ErrInvalidValue},
		{[]string{"--json", "--xml", "in.txt"}, ErrMutualExclusion},
		{[]string{"--auth", "--token", "abc", "in.txt"}, ErrDependency},
	}

	for _, tc := range cases {
		if err := p.Parse(tc.args); !errors.Is(err, tc.want) {
			t.Errorf("args=%v: expected %v, got %v", tc.args, tc.want, err)
		}
	}

	if err := p.Parse([]string{"-m", "b", "-l", "2", "-r", "1.5", "-f", "yaml", "--auth", "-t", "tk", "-u", "usr", "--json", "in.txt"}); err != nil {
		t.Fatalf("unexpected error on valid input: %v", err)
	}
}

func TestBadRuleFailsBeforeHelpAndInitErrors(t *testing.T) {
	var a bool
	p := New("app")
	p.BindBool("a", "alpha", &a, "A")
	p.AddExclusive("alpha, ghost_flag")
	if err := p.Parse([]string{"-h"}); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected ErrUnknownFlag for bad rule on -h, got %v", err)
	}

	p2 := New("app")
	p2.BindBool("a", "alpha", &a, "A")
	if err := p2.SetEnv("missing", "ENV_A"); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected ErrUnknownFlag from SetEnv, got %v", err)
	}
	if err := p2.Parse([]string{}); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected initErr propagated to Parse, got %v", err)
	}
}

func TestHelpVersionHiddenAndUTF8(t *testing.T) {
	var mode byte
	var secret bool
	var pos string
	var out bytes.Buffer

	p := New("/usr/bin/mytool")
	p.SetOutput(&out)
	p.SetVersion("2.5.0", "V", "git-abc123")
	p.BindChar("m", "mode", &mode, false, "ab", "Modalità operativa")
	p.BindBool("s", "secret", &secret, "Opzione segreta")
	p.BindPos("target", &pos, false, "", "File destinazione")

	if err := p.SetChoiceHelp("mode", "a", "Scelta alfa con accento è"); err != nil {
		t.Fatalf("SetChoiceHelp failed: %v", err)
	}
	if err := p.SetChoiceHelp("mode", "b", "Scelta beta"); err != nil {
		t.Fatalf("SetChoiceHelp failed: %v", err)
	}
	if err := p.SetHidden("secret"); err != nil {
		t.Fatalf("SetHidden failed: %v", err)
	}

	if err := p.Parse([]string{"--secret"}); err != nil || !secret {
		t.Fatalf("hidden flag failed: err=%v secret=%v", err, secret)
	}

	out.Reset()
	if err := p.Parse([]string{"foo.txt", "-h"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "Usage: mytool [OPTIONS]") ||
		strings.Contains(out.String(), "not recognized") ||
		strings.Contains(out.String(), "--secret") {
		t.Fatalf("unexpected help output:\n%s", out.String())
	}

	out.Reset()
	if err := p.Parse([]string{"-h", "mode"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "Scelta alfa") || !strings.Contains(out.String(), "Scelta beta") {
		t.Fatalf("missing choice helps in single help:\n%s", out.String())
	}

	out.Reset()
	if err := p.Parse([]string{"-h", "secret"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "not recognized") {
		t.Fatalf("hidden flag was exposed in single help:\n%s", out.String())
	}

	out.Reset()
	if err := p.Parse([]string{"-V"}); !errors.Is(err, ErrVersionRequested) {
		t.Fatalf("expected ErrVersionRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "mytool 2.5.0") || !strings.Contains(out.String(), "git-abc123") {
		t.Fatalf("unexpected version output:\n%s", out.String())
	}
}

func TestPanicsOnInvalidSetup(t *testing.T) {
	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}

	assertPanics("nil parser", func() {
		var p *Parser
		p.PrintHelp()
	})

	assertPanics("nil target", func() {
		p := New("app")
		p.BindInt("n", "num", nil, false, "", "Num")
	})

	assertPanics("pos after rest", func() {
		var rest []string
		var pos string
		p := New("app")
		p.BindRest("rest", &rest, false, "", "Rest")
		p.BindPos("pos", &pos, false, "", "Pos")
	})
}
