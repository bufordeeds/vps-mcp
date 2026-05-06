package vps

import (
	"strings"
	"testing"
	"time"
)

const sampleLog = `
{"ts":1714900000.0,"status":200,"request":{"remote_ip":"1.2.3.4","method":"GET","host":"example.com","uri":"/","headers":{"User-Agent":["Mozilla/5.0"]}}}
{"ts":1714900060.0,"status":404,"request":{"remote_ip":"1.2.3.4","method":"GET","host":"example.com","uri":"/missing","headers":{}}}
{"ts":1714900120.0,"status":200,"request":{"remote_ip":"5.6.7.8","method":"GET","host":"other.com","uri":"/","headers":{}}}
{"ts":1714899000.0,"status":200,"request":{"remote_ip":"9.9.9.9","method":"GET","host":"example.com","uri":"/old","headers":{}}}
not a json line
`

func TestFilterCaddyEntries_DomainAndCutoff(t *testing.T) {
	got := filterCaddyEntries(sampleLog, "example.com", 1714899500)
	if len(got) != 2 {
		t.Fatalf("got %d matches, want 2 (the two example.com entries after cutoff)", len(got))
	}
	if got[0].Ts > got[1].Ts {
		t.Errorf("entries should be sorted ascending by ts, got %v then %v", got[0].Ts, got[1].Ts)
	}
}

func TestFilterCaddyEntries_DomainCaseInsensitive(t *testing.T) {
	got := filterCaddyEntries(sampleLog, "Example.COM", 0)
	if len(got) != 3 {
		t.Errorf("case-insensitive host match: got %d, want 3", len(got))
	}
}

func TestFilterCaddyEntries_SkipsMalformed(t *testing.T) {
	got := filterCaddyEntries(sampleLog, "anything.com", 0)
	// "not a json line" must be silently skipped, not panic or pollute results.
	if len(got) != 0 {
		t.Errorf("got %d matches, want 0", len(got))
	}
}

func TestFormatCaddyEntries_RespectLimit(t *testing.T) {
	entries := filterCaddyEntries(sampleLog, "example.com", 0)
	args := parsedCaddyArgs{
		caddyLogsArgs: caddyLogsArgs{Domain: "example.com", Since: "1h", Limit: 2},
		window:        time.Hour,
	}
	out := formatCaddyEntries(entries, args)
	if strings.Count(out, "\n") != 2 {
		t.Errorf("expected header + 2 entry lines, got:\n%s", out)
	}
	if !strings.Contains(out, "showing 2") {
		t.Errorf("header should reflect truncation to limit; got:\n%s", out)
	}
}

func TestFormatCaddyEntries_EmptyMatches(t *testing.T) {
	args := parsedCaddyArgs{
		caddyLogsArgs: caddyLogsArgs{Domain: "nope.com", Since: "1h", Limit: 50},
		window:        time.Hour,
	}
	out := formatCaddyEntries(nil, args)
	if !strings.Contains(out, "No requests to nope.com") {
		t.Errorf("expected friendly empty message, got: %q", out)
	}
}

func TestParseCaddyArgs(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid", `{"domain":"example.com","since":"1h"}`, false},
		{"defaults applied", `{"domain":"example.com"}`, false},
		{"bad domain", `{"domain":"not a domain"}`, true},
		{"missing domain", `{}`, true},
		{"bad duration", `{"domain":"example.com","since":"forever"}`, true},
		{"negative duration", `{"domain":"example.com","since":"-1h"}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseCaddyArgs([]byte(c.raw))
			if (err != nil) != c.wantErr {
				t.Errorf("parseCaddyArgs(%s) err=%v, wantErr=%v", c.raw, err, c.wantErr)
			}
		})
	}
}
