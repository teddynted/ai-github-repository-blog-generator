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
	if cfg.RequireHumanApproval != DefaultRequireHumanApprove {
		t.Errorf("RequireHumanApproval = %v, want %v", cfg.RequireHumanApproval, DefaultRequireHumanApprove)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.SMTPHost != DefaultSMTPHost {
		t.Errorf("SMTPHost = %q, want %q", cfg.SMTPHost, DefaultSMTPHost)
	}
	if cfg.SMTPPort != DefaultSMTPPort {
		t.Errorf("SMTPPort = %d, want %d", cfg.SMTPPort, DefaultSMTPPort)
	}
}

func TestLoadReadsSMTP(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"SMTP_HOST":            "pro.eu.turbo-smtp.com",
		"SMTP_PORT":            "465",
		"SMTP_USERNAME":        "acct@example.com",
		"SMTP_PASSWORD_SECRET": "blog-gen/notifications/smtp-password",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SMTPHost != "pro.eu.turbo-smtp.com" || cfg.SMTPPort != 465 {
		t.Errorf("host/port = %q/%d", cfg.SMTPHost, cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "acct@example.com" {
		t.Errorf("username = %q", cfg.SMTPUsername)
	}
	if cfg.SMTPPasswordSecret != "blog-gen/notifications/smtp-password" {
		t.Errorf("password secret = %q", cfg.SMTPPasswordSecret)
	}
}

func TestLoadReadsValues(t *testing.T) {
	cfg, err := Load(envMap(map[string]string{
		"AWS_REGION":             "eu-west-1",
		"PUBLISH_TRIGGER":        "[blog]",
		"REPOSITORIES_TABLE":     "blog-gen-repositories",
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
	if !cfg.RequireHumanApproval {
		t.Error("RequireHumanApproval = false, want true")
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	for _, tc := range []struct{ name, key, val string }{
		{"non-bool approval", "REQUIRE_HUMAN_APPROVAL", "maybe"},
		{"non-numeric smtp port", "SMTP_PORT", "ssl"},
		{"out-of-range smtp port", "SMTP_PORT", "70000"},
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
