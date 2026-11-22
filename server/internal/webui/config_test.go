package webui

import "testing"

func TestResolveDataDirPrecedence(t *testing.T) {
	t.Setenv("LANPARTY_WEB_DATA_DIR", "/env/data")
	if got, _ := ResolveDataDir("/flag/data"); got != "/flag/data" {
		t.Fatalf("flag lost: %s", got)
	}
	if got, _ := ResolveDataDir(""); got != "/env/data" {
		t.Fatalf("env lost: %s", got)
	}
}

func TestResolveDataDirDefault(t *testing.T) {
	t.Setenv("LANPARTY_WEB_DATA_DIR", "")
	got, err := ResolveDataDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "./data" {
		t.Fatalf("default = %q", got)
	}
}

func TestConfigPathsShareDataDir(t *testing.T) {
	p := ConfigPaths("/srv/lan-party")
	if p.ConfigDir != "/srv/lan-party" || p.DataDir != "/srv/lan-party" {
		t.Fatalf("unexpected paths: %+v", p)
	}
}
