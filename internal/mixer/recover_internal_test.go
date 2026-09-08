package mixer

import (
	"errors"
	"strings"
	"testing"
)

func TestWithRecoverPassesResultThrough(t *testing.T) {
	out, err := withRecover(func() ([]byte, error) {
		return []byte("body"), nil
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if string(out) != "body" {
		t.Fatalf("out = %q", out)
	}
}

func TestWithRecoverPassesErrorThrough(t *testing.T) {
	want := errors.New("boom")
	if _, err := withRecover(func() ([]byte, error) { return nil, want }); !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
}

func TestWithRecoverTurnsPanicIntoError(t *testing.T) {
	out, err := withRecover(func() ([]byte, error) {
		var m map[string]any
		_ = m["missing"].(string) // panics: type assertion on nil
		return []byte("unreachable"), nil
	})
	if err == nil {
		t.Fatal("want error instead of panic")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("err = %v, want it to mention the panic", err)
	}
	if out != nil {
		t.Fatalf("out = %q, want nil so the caller keeps the original body", out)
	}
}
