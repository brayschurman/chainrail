package cmd

import (
	"bytes"
	"testing"
)

func TestSubmitCommandIsRegistered(t *testing.T) {
	if !commandRegistered("submit") {
		t.Fatal("submit command not registered with root")
	}
}

func TestSubmitCommandReturnsNotImplemented(t *testing.T) {
	rootCmd.SetArgs([]string{"submit"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
