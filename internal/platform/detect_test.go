package platform_test

import (
	"testing"

	"github.com/shuhi/dotfiles/internal/platform"
)

func TestDetectWSL(t *testing.T) {
	if !platform.DetectWSL("Fedora", "") {
		t.Fatal("env should detect WSL")
	}
	if !platform.DetectWSL("", "Linux version 6.6 Microsoft WSL2") {
		t.Fatal("microsoft should detect WSL")
	}
	if !platform.DetectWSL("", "contains WSL token") {
		t.Fatal("WSL should detect")
	}
	if !platform.DetectWSL("", "MICROSOFT") {
		t.Fatal("case insensitive")
	}
	if platform.DetectWSL("", "Linux version 6.8.0-generic") {
		t.Fatal("plain linux is not WSL")
	}
	if platform.DetectWSL("  ", "nothing") {
		t.Fatal("blank env is not WSL")
	}
}
