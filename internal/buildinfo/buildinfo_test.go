package buildinfo

import "testing"

func TestSummaryWithoutCommit(t *testing.T) {
	version, commit := Version, Commit
	defer func() { Version, Commit = version, commit }()

	Version, Commit = "0.1.0", "unknown"
	if got := Summary(); got != "0.1.0" {
		t.Fatalf("Summary() = %q, want %q", got, "0.1.0")
	}
}

func TestSummaryWithCommit(t *testing.T) {
	version, commit := Version, Commit
	defer func() { Version, Commit = version, commit }()

	Version, Commit = "0.1.0", "abc1234"
	if got := Summary(); got != "0.1.0 (abc1234)" {
		t.Fatalf("Summary() = %q, want %q", got, "0.1.0 (abc1234)")
	}
}
