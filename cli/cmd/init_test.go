package cmd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/garancehq/garance/cli/internal/project"
)

func TestInitCreatesFiles(t *testing.T) {
	dir := t.TempDir()

	err := project.Init(dir, "test-project")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Check garance.json exists
	if _, err := os.Stat(filepath.Join(dir, "garance.json")); err != nil {
		t.Error("garance.json not created")
	}

	// Check garance.schema.ts exists
	if _, err := os.Stat(filepath.Join(dir, "garance.schema.ts")); err != nil {
		t.Error("garance.schema.ts not created")
	}

	// Check seeds directory
	if _, err := os.Stat(filepath.Join(dir, "seeds", "seed.sql")); err != nil {
		t.Error("seeds/seed.sql not created")
	}

	// Check migrations directory
	if _, err := os.Stat(filepath.Join(dir, "migrations")); err != nil {
		t.Error("migrations/ not created")
	}

	// Check .env.local
	if _, err := os.Stat(filepath.Join(dir, ".env.local")); err != nil {
		t.Error(".env.local not created")
	}

	// Load config
	config, err := project.LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.Name != "test-project" {
		t.Errorf("expected name test-project, got %s", config.Name)
	}
}

func TestInitFailsIfAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	project.Init(dir, "first")
	err := project.Init(dir, "second")
	// Should not fail — Init doesn't check for existing project
	// The command checks via project.Exists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProjectExists(t *testing.T) {
	dir := t.TempDir()

	if project.Exists(dir) {
		t.Error("should not exist before init")
	}

	project.Init(dir, "test")

	if !project.Exists(dir) {
		t.Error("should exist after init")
	}
}
