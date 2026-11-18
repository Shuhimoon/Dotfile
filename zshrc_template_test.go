package dotfiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuhi/dotfiles/internal/shellrc"
)

func zshTemplate(t *testing.T) string {
	t.Helper()
	b, err := Configs.ReadFile("shells/zsh/zshrc")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The deployed ~/.zshrc must not carry machine-specific lines from the Mac it
// came from, and every plugin source / tool init is guarded.
func TestZshTemplateIsPortable(t *testing.T) {
	s := zshTemplate(t)
	var code []string
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			code = append(code, line)
		}
	}
	codeOnly := strings.Join(code, "\n")
	// Mac lines stay (Dotfiles ships a Mac build too), but nothing is tied to
	// one machine and Mac-only parts are guarded so Linux is unaffected.
	for _, bad := range []string{"/Users/", "/home/", "fanice"} {
		if strings.Contains(codeOnly, bad) {
			t.Errorf("template hardcodes %q", bad)
		}
	}
	guarded := func(guard, inner string) {
		t.Helper()
		g, i := strings.Index(codeOnly, guard), strings.Index(codeOnly, inner)
		if g < 0 || i < 0 || g > i || strings.Contains(codeOnly[g:i], "\nfi\n") {
			t.Errorf("%q is not inside %q", inner, guard)
		}
	}
	guarded("if [[ $OSTYPE == darwin* ]]; then", "export JAVA_HOME=")
	guarded("if [[ $OSTYPE == darwin* ]]; then", "export ANDROID_HOME=")
	guarded("if [[ -d $HOME/.grok ]]; then", "export PATH=\"$HOME/.grok/bin")
	guarded("[[ -d $HOME/develop/flutter/bin ]] &&", "develop/flutter/bin:$PATH")
	guarded("if command -v go &>/dev/null; then", "go env GOROOT")
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "source ~/.zsh/") && !strings.Contains(s, "-f "+strings.Fields(l)[1]) {
			t.Errorf("unguarded plugin source: %s", l)
		}
		if strings.HasPrefix(l, `eval "$(starship`) && !strings.Contains(s, "command -v starship") {
			t.Errorf("unguarded starship init")
		}
	}
	for _, want := range []string{
		"# dotfiles:provides base local-bin starship rust go",
		"typeset -U path PATH", "HISTFILE", "ssh-agent", "compinit", "bindkey",
		`eval "$(starship init zsh)"`, ".cargo/env", "~/.zsh/cache",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("template lacks %q", want)
		}
	}
	if i, j := strings.LastIndex(s, "starship init zsh"), strings.LastIndex(s, ".cargo/env"); i < j {
		t.Errorf("starship init must come last")
	}
}

func syntaxCheck(t *testing.T, shell, body string) {
	t.Helper()
	bin, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("%s not installed", shell)
	}
	p := filepath.Join(t.TempDir(), "rc")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "-n", p).CombinedOutput(); err != nil {
		t.Fatalf("%s -n: %v\n%s", shell, err, out)
	}
}

func TestZshTemplateSyntax(t *testing.T) {
	// The template plus every block the shell-rc step may append.
	all := []shellrc.BlockID{shellrc.BlockLocalBin, shellrc.BlockFzf, shellrc.BlockStarship, shellrc.BlockRust, shellrc.BlockGo, shellrc.BlockBase}
	out, _, err := shellrc.Ensure([]byte(zshTemplate(t)), shellrc.Zsh, all)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), shellrc.Begin(shellrc.BlockBase)) {
		t.Fatal("template provides base; it must not be appended")
	}
	syntaxCheck(t, "zsh", string(out))
}

func TestBashBaseBlockSyntax(t *testing.T) {
	fedora := "# .bashrc\nif [ -f /etc/bashrc ]; then\n    . /etc/bashrc\nfi\n"
	out, _, err := shellrc.Ensure([]byte(fedora), shellrc.Bash, []shellrc.BlockID{shellrc.BlockFzf, shellrc.BlockBase})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `eval "$(starship init bash)"`) {
		t.Fatalf("no starship init bash:\n%s", out)
	}
	syntaxCheck(t, "bash", string(out))
}
