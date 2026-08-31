package config

import (
	"os"
	"testing"
)

func TestCookieDomainDefault(t *testing.T) {
	d := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(d)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.CookieDomain != ".popsigner.com" {
		t.Fatalf("default = %q", cfg.Auth.CookieDomain)
	}
}

func TestCookieDomainEmptyOverride(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(d+"/config.yaml", []byte("auth:\n  cookie_domain: \"\"\n"), 0o644)
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(d)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.CookieDomain != "" {
		t.Fatalf("empty override lost, got %q", cfg.Auth.CookieDomain)
	}
}

func TestCookieDomainEnvOverride(t *testing.T) {
	d := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(d)
	t.Setenv("BANHBAO_AUTH_COOKIE_DOMAIN", ".example.org")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Auth.CookieDomain != ".example.org" {
		t.Fatalf("env override lost, got %q", cfg.Auth.CookieDomain)
	}
}
