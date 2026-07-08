package config

import (
	"strings"
	"testing"
)

// envMap returns a Getenv backed by a map, so tests never touch process state.
func envMap(m map[string]string) Getenv {
	return func(k string) string { return m[k] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(envMap(nil))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.PublishTrigger != DefaultPublishTrigger {
		t.Errorf("PublishTrigger = %q, want %q", cfg.PublishTrigger, DefaultPublishTrigger)
	}
	if cfg.SecretsPrefix != DefaultSecretsPrefix {
		t.Errorf("SecretsPrefix = %q, want %q", cfg.SecretsPrefix, DefaultSecretsPrefix)
	}
	if cfg.OllamaModel != DefaultOllamaModel {
		t.Errorf("OllamaModel = %q, want %q", cfg.OllamaModel, DefaultOllamaModel)
	}
	if cfg.IdleTimeoutMinutes != DefaultIdleTimeoutMinutes {
		t.Errorf("IdleTimeoutMinutes = %d, want %d", cfg.IdleTimeoutMinutes, DefaultIdleTimeoutMinutes)
	}
	if cfg.RequireHumanApproval != DefaultRequireHumanApprove {
		t.Errorf("RequireHumanApproval = %v, want %v", cfg.RequireHumanApproval, DefaultRequireHumanApprove)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
}

func TestLoadReadsValues(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"AWS_REGION":             "eu-west-1",
		"PUBLISH_TRIGGER":        "[blog]",
		"REPOSITORIES_TABLE":     "blog-gen-repositories",
		"IDLE_TIMEOUT_MINUTES":   "30",
		"REQUIRE_HUMAN_APPROVAL": "true",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AWSRegion != "eu-west-1" {
		t.Errorf("AWSRegion = %q", cfg.AWSRegion)
	}
	if cfg.PublishTrigger != "[blog]" {
		t.Errorf("PublishTrigger = %q", cfg.PublishTrigger)
	}
	if cfg.IdleTimeoutMinutes != 30 {
		t.Errorf("IdleTimeoutMinutes = %d", cfg.IdleTimeoutMinutes)
	}
	if !cfg.RequireHumanApproval {
		t.Error("RequireHumanApproval = false, want true")
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	for _, tc := range []struct{ name, key, val string }{
		{"non-numeric idle timeout", "IDLE_TIMEOUT_MINUTES", "soon"},
		{"non-positive idle timeout", "IDLE_TIMEOUT_MINUTES", "0"},
		{"non-bool approval", "REQUIRE_HUMAN_APPROVAL", "maybe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(envMap(map[string]string{tc.key: tc.val})); err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
		})
	}
}

func TestRequireReportsMissing(t *testing.T) {
	cfg, _ := Load(envMap(map[string]string{"AWS_REGION": "us-east-1"}))
	err := cfg.Require("AWSRegion", "RepositoriesTable", "EventBusName")
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
	msg := err.Error()
	if !strings.Contains(msg, "RepositoriesTable") || !strings.Contains(msg, "EventBusName") {
		t.Errorf("error should list missing fields, got %q", msg)
	}
	if strings.Contains(msg, "AWSRegion") {
		t.Errorf("present field should not be reported missing, got %q", msg)
	}
}

func TestRequirePassesWhenPresent(t *testing.T) {
	cfg, _ := Load(envMap(map[string]string{
		"AWS_REGION":         "us-east-1",
		"REPOSITORIES_TABLE": "t",
		"EVENT_BUS_NAME":     "b",
	}))
	if err := cfg.Require("AWSRegion", "RepositoriesTable", "EventBusName"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireRejectsUnknownField(t *testing.T) {
	cfg, _ := Load(envMap(nil))
	if err := cfg.Require("NotAField"); err == nil {
		t.Fatal("expected error for unknown field name")
	}
}
