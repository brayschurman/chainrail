package cmd

import (
	"bytes"
	"testing"
)

func TestAddCommandIsRegistered(t *testing.T) {
	if !commandRegistered("add") {
		t.Fatal("add command not registered with root")
	}
}

func TestAddCommandReturnsNotImplemented(t *testing.T) {
	rootCmd.SetArgs([]string{"add", "foo"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddCommandRequiresOneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"add"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no arg passed, got nil")
	}
}
