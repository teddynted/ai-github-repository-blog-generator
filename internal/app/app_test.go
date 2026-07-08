package app

import (
	"io"
	"testing"
)

func TestNewFromBootstrapsConfigAndLogger(t *testing.T) {
	env := map[string]string{
		"AWS_REGION":      "us-east-1",
		"PUBLISH_TRIGGER": "blog:",
		"LOG_LEVEL":       "debug",
	}
	a, err := newFrom(func(k string) string { return env[k] }, io.Discard)
	if err != nil {
		t.Fatalf("newFrom returned error: %v", err)
	}
	if a.Config.AWSRegion != "us-east-1" {
		t.Errorf("config not loaded: %+v", a.Config)
	}
	if a.Logger == nil {
		t.Fatal("logger was not constructed")
	}
	// Logger must be usable immediately.
	a.Logger.Debug("bootstrap ok")
}

func TestNewFromPropagatesConfigError(t *testing.T) {
	env := map[string]string{"IDLE_TIMEOUT_MINUTES": "not-a-number"}
	if _, err := newFrom(func(k string) string { return env[k] }, io.Discard); err == nil {
		t.Fatal("expected bootstrap to fail on malformed config")
	}
}
