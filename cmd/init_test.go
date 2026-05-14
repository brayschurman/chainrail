package cmd

import (
	"bytes"
	"testing"
)

func TestInitCommandIsRegistered(t *testing.T) {
	if !commandRegistered("init") {
		t.Fatal("init command not registered with root")
	}
}

func TestInitCommandReturnsNotImplemented(t *testing.T) {
	rootCmd.SetArgs([]string{"init"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
