package config_test

import (
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/config"
)

func TestStringEnv(t *testing.T) {
	t.Setenv("SUBMIX_TEST_STR", "value")
	if got := config.String("SUBMIX_TEST_STR", "fallback"); got != "value" {
		t.Fatalf("got %q", got)
	}
	if got := config.String("SUBMIX_TEST_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

func TestMustStringEnv(t *testing.T) {
	t.Setenv("SUBMIX_TEST_REQ", "x")
	v, err := config.MustString("SUBMIX_TEST_REQ")
	if err != nil || v != "x" {
		t.Fatalf("v=%q err=%v", v, err)
	}
	if _, err := config.MustString("SUBMIX_TEST_ABSENT"); err == nil {
		t.Fatal("want error for missing variable")
	}
}

func TestDurationEnv(t *testing.T) {
	t.Setenv("SUBMIX_TEST_DUR", "750ms")
	d, err := config.Duration("SUBMIX_TEST_DUR", time.Second)
	if err != nil || d != 750*time.Millisecond {
		t.Fatalf("d=%v err=%v", d, err)
	}

	d, err = config.Duration("SUBMIX_TEST_DUR_MISSING", 5*time.Minute)
	if err != nil || d != 5*time.Minute {
		t.Fatalf("d=%v err=%v", d, err)
	}

	t.Setenv("SUBMIX_TEST_DUR_BAD", "abc")
	if _, err := config.Duration("SUBMIX_TEST_DUR_BAD", time.Second); err == nil {
		t.Fatal("want error for malformed duration")
	}
}
