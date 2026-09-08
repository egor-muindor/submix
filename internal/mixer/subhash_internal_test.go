package mixer

import "testing"

func TestSubHashIsDeterministicAndHidesInput(t *testing.T) {
	const shortUUID = "abc123-secret-short-uuid"

	got := subHash(shortUUID)
	again := subHash(shortUUID)
	if got != again {
		t.Fatalf("subHash must be deterministic: %q != %q", got, again)
	}
	if got == "" {
		t.Fatal("subHash must not be empty")
	}
	if got == shortUUID {
		t.Fatalf("subHash must not just echo the input: %q", got)
	}
	for i := 0; i+len(shortUUID) <= len(got); i++ {
		if got[i:i+len(shortUUID)] == shortUUID {
			t.Fatalf("subHash output must not contain the original shortUuid: %q", got)
		}
	}

	if other := subHash("different-uuid"); other == got {
		t.Fatalf("different inputs must not collide: both hashed to %q", got)
	}
}
