package main

import (
	"os"
	"regexp"
	"testing"
)

func TestFlagsOnEitherSideOfTheCommand(t *testing.T) {
	for _, args := range [][]string{
		{"sync", "--verbose", "--socket", "/s"},
		{"--verbose", "sync", "--socket=/s"},
		{"-v", "--socket", "/s", "sync"},
	} {
		o, err := parseArgs(args)
		if err != nil || o.command != "sync" || !o.verbose || o.socket != "/s" {
			t.Errorf("%v: %+v %v", args, o, err)
		}
	}
}

func TestClearConfigBelongsToClear(t *testing.T) {
	if o, err := parseArgs([]string{"clear", "--config"}); err != nil || !o.clearConfig {
		t.Fatalf("%+v %v", o, err)
	}
	if _, err := parseArgs([]string{"sync", "--config"}); err == nil {
		t.Fatal("--config accepted on sync")
	}
}

func TestUnknownFlagsAndExtraArgumentsAreRejected(t *testing.T) {
	for _, args := range [][]string{{"sync", "--nope"}, {"sync", "list"}, {"--socket"}} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}

// The install fetches the release tagged with the manifest's version, so the
// binary inside it has to agree.
func TestVersionMatchesTheManifest(t *testing.T) {
	data, err := os.ReadFile("../../herdr-plugin.toml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`).FindSubmatch(data)
	if m == nil || string(m[1]) != Version {
		t.Fatalf("manifest version %q, binary %q", m, Version)
	}
}
