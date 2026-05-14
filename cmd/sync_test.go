package cmd

import (
	"bytes"
	"testing"
)

func TestSyncCommandIsRegistered(t *testing.T) {
	if !commandRegistered("sync") {
		t.Fatal("sync command not registered with root")
	}
}

func TestSyncCommandReturnsNotImplemented(t *testing.T) {
	rootCmd.SetArgs([]string{"sync"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
