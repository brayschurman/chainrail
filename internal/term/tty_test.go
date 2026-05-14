package term

import (
	"bytes"
	"os"
	"testing"
)

func TestIsTTY_NilWriter(t *testing.T) {
	if IsTTY(nil) {
		t.Fatal("nil writer should not be a TTY")
	}
}

func TestIsTTY_BytesBuffer(t *testing.T) {
	var buf bytes.Buffer
	if IsTTY(&buf) {
		t.Fatal("*bytes.Buffer should not be a TTY")
	}
}

func TestIsTTY_RegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "ttytest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if IsTTY(f) {
		t.Fatal("regular file should not be a TTY")
	}
}

func TestIsTTY_PipeIsNotTTY(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()
	if IsTTY(pw) {
		t.Fatal("os.Pipe writer should not be a TTY")
	}
}
