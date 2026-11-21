package relay

import "testing"

func TestGeneratePassword(t *testing.T) {
	first, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	second, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 32 {
		t.Fatalf("generated password length = %d, want 32", len(first))
	}
	if first == second {
		t.Fatal("two generated passwords are identical")
	}
}
