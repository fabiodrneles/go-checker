package main

import (
	"path/filepath"
	"testing"

	"github.com/fabiodrneles/go-checker/internal/config"
)

// 008 AC-1: every example file is valid.
func TestExamplesAreValid(t *testing.T) {
	files, err := filepath.Glob("examples/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, f := range files {
		if filepath.Base(f) == "openapi.yaml" {
			continue // an OpenAPI input for init, not a check file
		}
		if _, err := config.Load(f); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}
