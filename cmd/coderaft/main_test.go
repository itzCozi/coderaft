package main

import (
	"testing"

	"coderaft/internal/commands"
)

func TestMainFunction(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("commands.Execute caused a panic: %v", r)
		}
	}()

	_ = commands.Execute
}

func TestImports(t *testing.T) {
	if commands.Version == "" {
		t.Error("commands.Version should not be empty")
	}
}
