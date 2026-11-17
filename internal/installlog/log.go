package installlog

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Item struct {
	ID     string   `json:"id"`
	Method string   `json:"method"`
	Argv   []string `json:"argv,omitempty"`
	Status string   `json:"status"`
	Note   string   `json:"note,omitempty"`
	Err    string   `json:"err,omitempty"`
}

type Record struct {
	Schema int    `json:"schema"`
	Time   string `json:"time"`
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	PM     string `json:"pm"`
	DryRun bool   `json:"dry_run"`
	Items  []Item `json:"items"`
}

// Write appends install.log and replaces last-install.json.
// dry_run in the file is always false. Schema 0 becomes 1.
func Write(dir string, rec Record, verbose bool, stderr io.Writer) error {
	if rec.Schema == 0 {
		rec.Schema = 1
	}
	rec.DryRun = false
	if rec.Time == "" {
		rec.Time = time.Now().UTC().Format(time.RFC3339)
	}
	if err := validate(rec); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := rejectPayload(body); err != nil {
		return err
	}
	var logBuf strings.Builder
	for _, it := range rec.Items {
		fmt.Fprintf(&logBuf, "%s %s %s %s", rec.Time, it.ID, it.Method, it.Status)
		if it.Err != "" {
			fmt.Fprintf(&logBuf, " %s", it.Err)
		}
		logBuf.WriteByte('\n')
		if verbose && stderr != nil && len(it.Argv) > 0 {
			fmt.Fprintln(stderr, strings.Join(it.Argv, " "))
		}
	}
	if err := rejectPayload([]byte(logBuf.String())); err != nil {
		return err
	}
	logPath := filepath.Join(dir, "install.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(logBuf.String())
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	if cerr != nil {
		return cerr
	}
	return os.WriteFile(filepath.Join(dir, "last-install.json"), body, 0o644)
}

func validate(rec Record) error {
	fields := []string{rec.Time, rec.GOOS, rec.GOARCH, rec.PM}
	for _, it := range rec.Items {
		fields = append(fields, it.ID, it.Method, it.Status, it.Note, it.Err)
		fields = append(fields, it.Argv...)
	}
	for _, f := range fields {
		if strings.ContainsAny(f, "\n\r") {
			return fmt.Errorf("日誌欄位含有換行")
		}
		if strings.Contains(f, `eval "$(starship`) || strings.Contains(f, "# dotfiles:") {
			return fmt.Errorf("日誌不得包含 bashrc 片段")
		}
	}
	return nil
}

func rejectPayload(b []byte) error {
	s := string(b)
	if strings.Contains(s, `eval "$(starship`) || strings.Contains(s, "# dotfiles:") {
		return fmt.Errorf("日誌不得包含 bashrc 片段")
	}
	return nil
}
