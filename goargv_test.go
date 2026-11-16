package goargv

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestBoolNegationAndAllLiterals(t *testing.T) {
	var b1, b2 bool
	var s string
	b2 = true

	p := New("app")
	p.BindBool("a", "alpha", &b1, "Alpha")
	p.BindBool("b", "beta", &b2, "Beta")
	p.BindStr("s", "str", &s, false, "", "Str")

	truthy := []string{"1", "true", "TRUE", "yes", "Yes", "on", "ON"}
	for _, val := range truthy {
		b1 = false
		if err := p.Parse([]string{"--alpha=" + val}); err != nil || !b1 {
			t.Fatalf("truthy %q failed: err=%v b1=%v", val, err, b1)
		}
	}

	falsy := []string{"0", "false", "FALSE", "no", "No", "off", "OFF"}
	for _, val := range falsy {
		b1 = true
		if err := p.Parse([]string{"--alpha=" + val}); err != nil || b1 {
			t.Fatalf("falsy %q failed: err=%v b1=%v", val, err, b1)
		}
	}

	if err := p.Parse([]string{"--no-beta"}); err != nil || b2 {
		t.Fatalf("--no-beta failed: err=%v b2=%v", err, b2)
	}

	if err := p.Parse([]string{"--no-str"}); !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected ErrUnknownFlag for --no-str, got %v", err)
	}

	if err := p.Parse([]string{"--alpha=maybe"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue for invalid bool, got %v", err)
	}
}

func TestEdgeCasesTokensAndTypes(t *testing.T) {
	var str1, str2, pos1, pos2 string
	var rest []string
	var num int
	var flt float64
	var ch byte

	p := New("app")
	p.BindStr("s", "str", &str1, false, "", "S1")
	p.BindStr("k", "kv", &str2, false, "", "S2")
	p.BindInt("n", "num", &num, false, "-5, 0, 10", "Int")
	p.BindFloat("f", "flt", &flt, false, "1.0, -0.5", "Float")
	p.BindChar("c", "char", &ch, false, "a,b", "Char")
	p.BindPos("p1", &pos1, true, "", "Pos1")
	p.BindPos("p2", &pos2, false, "", "Pos2")
	p.BindRest("rest", &rest, false, "", "Rest")

	args := []string{
		"--str=",
		"-ka=b=c",
		"-n", "-5",
		"-f", "1",
		"-c", "a",
		"first",
		"",
		"--",
		"-h", "--version", "-5",
	}
	if err := p.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if str1 != "" || str2 != "a=b=c" || num != -5 || flt != 1.0 || ch != 'a' ||
		pos1 != "first" || pos2 != "" || len(rest) != 3 || rest[0] != "-h" || rest[2] != "-5" {
		t.Fatalf("unexpected parsed state: str1=%q str2=%q num=%d flt=%v ch=%c pos1=%q pos2=%q rest=%v",
			str1, str2, num, flt, ch, pos1, pos2, rest)
	}

	if err := p.Parse([]string{"-n", "-.5", "p1"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue for '-n -.5', got %v", err)
	}

	if err := p.Parse([]string{"-n", "-abc", "p1"}); !errors.Is(err, ErrMissingValue) {
		t.Fatalf("expected ErrMissingValue for '-n -abc', got %v", err)
	}

	if err := p.Parse([]string{"-c", "ab", "p1"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue for multi-byte char, got %v", err)
	}
	if err := p.Parse([]string{"-c", "", "p1"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue for empty char, got %v", err)
	}
}

func TestAtomicityOnParseAndDefaultErrors(t *testing.T) {
	port := 0
	mode := byte('x')

	p := New("app")
	p.BindInt("p", "port", &port, false, "80,443", "Port")
	p.BindChar("m", "mode", &mode, false, "abc", "Mode")

	if err := p.SetDefault("port", "80"); err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}

	if err := p.Parse([]string{"--port", "9999"}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
	if port != 80 {
		t.Fatalf("port was mutated on failed Parse: got %d, want 80", port)
	}

	p2 := New("app")
	p2.BindInt("p", "port", &port, false, "80,443", "Port")
	if err := p2.SetDefault("port", "not_int"); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue from SetDefault, got %v", err)
	}
	if err := p2.Parse([]string{}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected initErr propagated to Parse, got %v", err)
	}
}

func TestEnvInteractionsWithRulesAndLists(t *testing.T) {
	var json, xml, auth, reqFlag bool
	var token string
	var port int
	var inc []string

	p := New("app")
	p.SetEnvPrefix("MYAPP")
	p.BindBool("j", "json", &json, "JSON")
	p.BindBool("x", "xml", &xml, "XML")
	p.BindBool("a", "auth", &auth, "Auth")
	p.BindStr("t", "token", &token, false, "", "Token")
	p.BindInt("p", "port", &port, false, "", "Port")
	p.BindBool("r", "req", &reqFlag, "Req")
	p.BindStrList("I", "inc", &inc, false, "a,b,c", "Inc")

	_ = p.SetDefault("port", "8080")
	_ = p.SetDefault("inc", "a")
	_ = p.SetEnv("req", "EXPLICIT_REQ")

	p.AddExclusive("json, xml")
	p.AddDependency("auth", "token")

	t.Setenv("MYAPP_PORT", "")
	if err := p.Parse([]string{}); err != nil || port != 8080 {
		t.Fatalf("empty env should keep default 8080: err=%v port=%d", err, port)
	}

	t.Setenv("MYAPP_XML", "true")
	if err := p.Parse([]string{"--json"}); !errors.Is(err, ErrMutualExclusion) {
		t.Fatalf("expected ErrMutualExclusion across CLI+Env, got %v", err)
	}
	t.Setenv("MYAPP_XML", "")

	t.Setenv("MYAPP_TOKEN", "secret")
	if err := p.Parse([]string{"--auth"}); err != nil || !auth || token != "secret" {
		t.Fatalf("expected dependency satisfied via Env: err=%v auth=%v token=%q", err, auth, token)
	}

	t.Setenv("EXPLICIT_REQ", "invalid_bool")
	if err := p.Parse([]string{}); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue for invalid bool env, got %v", err)
	}
	t.Setenv("EXPLICIT_REQ", "")

	t.Setenv("MYAPP_INC", "b")
	if err := p.Parse([]string{}); err != nil || len(inc) != 1 || inc[0] != "b" {
		t.Fatalf("expected StrList from env to override default: err=%v inc=%v", err, inc)
	}
}

func TestHelpLayoutAlignmentAndWrapping(t *testing.T) {
	var format, margin, emptyDesc string
	var open bool = true
	var out bytes.Buffer

	p := New("") 
	p.SetOutput(nil) 
	p.SetOutput(&out)
	p.SetVersion("1.0.0", "V", "extra")
	p.SetDescription("  Tool description here.\nSecond line.  ")

	p.BindBool("", "open", &open, "Open browser")
	p.BindStr("", "pdf-format", &format, false, "", "Page format")
	p.BindStr("", "pdf-margin", &margin, false, "none,narrow,normal,wide", "Page margins with a fairly long description that should wrap cleanly across multiple lines")
	p.BindStr("", "no-desc", &emptyDesc, false, "", "")

	if err := p.Parse([]string{"--no-open"}); err != nil || open {
		t.Fatalf("unexpected parse error: %v", err)
	}

	p.PrintHelp()
	helpText := out.String()

	if !strings.Contains(helpText, "program v1.0.0\n\nTool description here.\nSecond line.\n\nUsage: program [OPTIONS]") {
		t.Fatalf("header/description order mismatch:\n%s", helpText)
	}
	if !strings.Contains(helpText, "--[no-]open") {
		t.Fatalf("expected --[no-]open even after target mutated to false:\n%s", helpText)
	}

	if !strings.Contains(helpText, "      --pdf-format <str>  Page format\n") {
		t.Fatalf("expected exact 2-space gutter after --pdf-format <str>:\n%s", helpText)
	}

	expectedWrap := "      --pdf-margin {none,narrow,normal,wide}\n" + strings.Repeat(" ", 26) + "Page margins"
	if !strings.Contains(helpText, expectedWrap) {
		t.Fatalf("expected --pdf-margin description to wrap to next line at col 26:\n%s", helpText)
	}

	if !strings.Contains(helpText, "      --no-desc <str>\n") {
		t.Fatalf("expected no trailing spaces for empty description:\n%s", helpText)
	}
}

func TestSingleHelpLookupModes(t *testing.T) {
	var port int
	var mode byte
	var secret bool
	var target string
	var out bytes.Buffer

	p := New("tool")
	p.SetOutput(&out)
	p.SetVersion("3.0.0", "V", "")
	p.BindInt("p", "port", &port, false, "", "Listen port")
	p.BindChar("m", "mode", &mode, false, "abc", "Mode")
	p.BindBool("s", "secret", &secret, "Hidden")
	p.BindPos("target", &target, false, "", "Target file")

	_ = p.SetDefault("port", "8080")
	_ = p.SetChoiceHelp("mode", "a", "Alpha")
	_ = p.SetHidden("secret")

	for _, query := range []string{"--port", "port", "-p", "p"} {
		out.Reset()
		if err := p.Parse([]string{"-h", query}); !errors.Is(err, ErrHelpRequested) {
			t.Fatalf("query %q: expected ErrHelpRequested, got %v", query, err)
		}
		if !strings.Contains(out.String(), "Long flag:             --port") {
			t.Fatalf("query %q: expected single help for --port, got:\n%s", query, out.String())
		}
	}

	for _, query := range []string{"version", "--version", "-V"} {
		out.Reset()
		if err := p.Parse([]string{"--help", query}); !errors.Is(err, ErrHelpRequested) {
			t.Fatalf("query %q: expected ErrHelpRequested, got %v", query, err)
		}
		if !strings.Contains(out.String(), "Display version information") {
			t.Fatalf("query %q: expected single help for version, got:\n%s", query, out.String())
		}
	}

	out.Reset()
	if err := p.Parse([]string{"-h", "target"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "Positional argument:   <target>") {
		t.Fatalf("expected positional single help, got:\n%s", out.String())
	}

	out.Reset()
	if err := p.Parse([]string{"-h", "--help"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if strings.Contains(out.String(), "not recognized") {
		t.Fatalf("'-h --help' should print full help without error:\n%s", out.String())
	}

	out.Reset()
	if err := p.Parse([]string{"-h", "--", "port"}); !errors.Is(err, ErrHelpRequested) {
		t.Fatalf("expected ErrHelpRequested, got %v", err)
	}
	if !strings.Contains(out.String(), "Usage: tool [OPTIONS]") {
		t.Fatalf("expected full help for '-h -- port', got:\n%s", out.String())
	}
}

func TestDefaultDisplayEscapes(t *testing.T) {
	if got := formatDefaultDisplay(""); got != `""` {
		t.Errorf(`formatDefaultDisplay("") = %q`, got)
	}
	if got := formatDefaultDisplay(" "); got != `' '` {
		t.Errorf(`formatDefaultDisplay(" ") = %q`, got)
	}
	if got := formatDefaultDisplay("\n\t\r"); got != `\n\t\r` {
		t.Errorf(`formatDefaultDisplay("\n\t\r") = %q`, got)
	}
}

func FuzzParse(f *testing.F) {
	f.Add("-h")
	f.Add("--port=8080")
	f.Add("-vvv")
	f.Add("--no-open")
	f.Add("--")
	f.Add("-p-12")
	f.Add("--=")
	f.Add("-")

	f.Fuzz(func(t *testing.T, a string) {
		var open, quiet bool
		var count, port int
		var flt float64
		var ch byte
		var str string
		var list, rest []string

		p := New("fuzz")
		p.SetOutput(&bytes.Buffer{})
		p.SetVersion("1.0", "V", "")
		p.BindBool("o", "open", &open, "Open")
		p.BindBool("q", "quiet", &quiet, "Quiet")
		p.BindCount("v", "verbose", &count, "Verbose")
		p.BindInt("p", "port", &port, false, "", "Port")
		p.BindFloat("f", "flt", &flt, false, "", "Float")
		p.BindChar("c", "char", &ch, false, "abc", "Char")
		p.BindStr("s", "str", &str, false, "", "Str")
		p.BindStrList("I", "inc", &list, false, "", "Inc")
		p.BindRest("rest", &rest, false, "", "Rest")
		p.AddExclusive("open, quiet")

		_ = p.Parse(strings.Split(a, " "))
	})
}
