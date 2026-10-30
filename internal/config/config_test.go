package config

import "testing"

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "SECRET_KEY"} {
		t.Setenv(k, "")
	}
}

func TestLoadFlags(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{"-a", ":9000", "-d", "dsn", "-r", "http://acc"})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{RunAddress: ":9000", DatabaseURI: "dsn", AccrualAddress: "http://acc", SecretKey: DefaultSecretKey}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{"-d", "dsn", "-r", "http://acc"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RunAddress != defaultRunAddress {
		t.Fatalf("RunAddress = %q", cfg.RunAddress)
	}
}

func TestLoadEnvOverridesFlags(t *testing.T) {
	clearEnv(t)
	t.Setenv("RUN_ADDRESS", ":7000")
	t.Setenv("DATABASE_URI", "env-dsn")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://env-acc")
	t.Setenv("SECRET_KEY", "env-secret")
	cfg, err := Load([]string{"-a", ":9000", "-d", "dsn", "-r", "http://acc"})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{RunAddress: ":7000", DatabaseURI: "env-dsn", AccrualAddress: "http://env-acc", SecretKey: "env-secret"}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no database", []string{"-r", "http://acc"}},
		{"no accrual", []string{"-d", "dsn"}},
		{"unknown flag", []string{"-d", "dsn", "-r", "http://acc", "-x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			if _, err := Load(tt.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
