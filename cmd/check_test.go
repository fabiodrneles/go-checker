package cmd

import (
	"bytes"
	"os"
	"testing"
)

func TestCheckCmd_TimeoutFlag(t *testing.T) {
	// Create a temporary config file for testing
	tmpFile := "test_config.yaml"
	err := os.WriteFile(tmpFile, []byte("sites:\n  - url: https://example.com\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "valid timeout 30",
			args:        []string{"check", "--timeout", "30", "-f", tmpFile},
			expectError: false,
		},
		{
			name:        "default timeout",
			args:        []string{"check", "-f", tmpFile},
			expectError: false,
		},
		{
			name:        "invalid timeout negative",
			args:        []string{"check", "--timeout", "-5", "-f", tmpFile},
			expectError: true,
		},
		{
			name:        "invalid timeout zero",
			args:        []string{"check", "--timeout", "0", "-f", tmpFile},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := RootCmd
			rootCmd.SetArgs(tt.args)

			var buf bytes.Buffer
			rootCmd.SetOut(&buf)
			rootCmd.SetErr(&buf)

			err := rootCmd.Execute()

			t.Logf("Test %s: err=%v, output=%s", tt.name, err, buf.String())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for invalid timeout, got nil. Output: %s", buf.String())
				}
			} else {
				if err != nil && !isConfigError(err) {
					t.Errorf("Unexpected error: %v. Output: %s", err, buf.String())
				}
			}
		})
	}
}

func TestCheckCmd_TimeoutValidation(t *testing.T) {
	// Test that negative timeout is rejected before config file check
	rootCmd := RootCmd
	rootCmd.SetArgs([]string{"check", "--timeout", "-1"})

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()

	if err == nil {
		t.Error("Expected error for negative timeout")
	}
}

func isConfigError(err error) bool {
	return err != nil && (bytes.Contains([]byte(err.Error()), []byte("config")) ||
		bytes.Contains([]byte(err.Error()), []byte("configura")))
}