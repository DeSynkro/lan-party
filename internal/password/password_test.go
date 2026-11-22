package password

import "testing"

func TestHashVerify(t *testing.T) {
	salt, hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if salt == "" || hash == "" {
		t.Fatal("empty salt or hash")
	}
	if !Verify("correct horse battery staple", salt, hash) {
		t.Fatal("valid password rejected")
	}
	if Verify("wrong", salt, hash) {
		t.Fatal("wrong password accepted")
	}
}

func TestHashUsesFreshSalt(t *testing.T) {
	salt1, hash1, err := Hash("same")
	if err != nil {
		t.Fatal(err)
	}
	salt2, hash2, err := Hash("same")
	if err != nil {
		t.Fatal(err)
	}
	if salt1 == salt2 || hash1 == hash2 {
		t.Fatal("salt or hash reused")
	}
}

func TestEmptyPasswordRejected(t *testing.T) {
	if _, _, err := Hash(""); err == nil {
		t.Fatal("empty password accepted")
	}
}

func TestGenerateLength(t *testing.T) {
	got, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 24 {
		t.Fatalf("generated password too short: %d", len(got))
	}
}
