package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/backup"
)

func TestRunnerParsesBackupCommandsAndRendersList(t *testing.T) {
	t.Parallel()
	service := &fakeService{result: app.Result{Backups: []backup.Info{{
		ID: "backup-1", CreatedAt: time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC),
		Reason: "manual", AppVersion: "1.2.3", DatabaseSchemaVersion: 3, FileCount: 4, TotalSize: 120,
	}}}}
	var stdout, stderr bytes.Buffer
	code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"backup", "list"})
	if code != ExitOK || len(service.commands) != 1 {
		t.Fatalf("Run() = %d commands=%d stderr=%q", code, len(service.commands), stderr.String())
	}
	command := service.commands[0]
	if command.Action != app.ActionBackup || command.BackupOperation != "list" {
		t.Fatalf("command = %+v", command)
	}
	for _, want := range []string{"backup-1", "reason=manual", "schema=3", "files=4", "bytes=120", "version=1.2.3"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunnerRequiresAndForwardsRestoreConfirmation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		args        []string
		confirmer   *fakeConfirmer
		wantCalls   int
		wantConfirm bool
		wantOutput  string
	}{
		{name: "yes flag", args: []string{"backup", "restore", "backup-1", "--yes"}, wantCalls: 1, wantConfirm: true},
		{name: "interactive yes", args: []string{"backup", "restore", "backup-1"}, confirmer: &fakeConfirmer{answer: true}, wantCalls: 1, wantConfirm: true},
		{name: "interactive no", args: []string{"backup", "restore", "backup-1"}, confirmer: &fakeConfirmer{}, wantOutput: "Restore canceled."},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeService{result: app.Result{Message: "restored"}}
			var stdout, stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)
			if test.confirmer != nil {
				runner = runner.WithConfirmer(test.confirmer)
			}
			if code := runner.Run(context.Background(), test.args); code != ExitOK {
				t.Fatalf("Run() = %d stderr=%q", code, stderr.String())
			}
			if len(service.commands) != test.wantCalls {
				t.Fatalf("service calls = %d, want %d", len(service.commands), test.wantCalls)
			}
			if test.wantCalls == 1 {
				command := service.commands[0]
				if command.BackupID != "backup-1" || command.BackupOperation != "restore" || command.BackupConfirmed != test.wantConfirm {
					t.Fatalf("command = %+v", command)
				}
			}
			if test.wantOutput != "" && !strings.Contains(stdout.String(), test.wantOutput) {
				t.Errorf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestTextConfirmerIsConservative(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  bool
	}{{"yes\n", true}, {"Y\n", true}, {"no\n", false}, {"\n", false}} {
		var output bytes.Buffer
		confirmed, err := NewTextConfirmer(strings.NewReader(test.input), &output).Confirm("Proceed? ")
		if err != nil || confirmed != test.want || output.String() != "Proceed? " {
			t.Errorf("Confirm(%q) = (%v, %v), output %q", test.input, confirmed, err, output.String())
		}
	}
	if _, err := NewTextConfirmer(strings.NewReader(""), &bytes.Buffer{}).Confirm("Proceed? "); err != io.EOF {
		t.Errorf("Confirm(EOF) error = %v, want EOF", err)
	}
}

type fakeConfirmer struct {
	answer bool
	err    error
	prompt string
}

func (confirmer *fakeConfirmer) Confirm(prompt string) (bool, error) {
	confirmer.prompt = prompt
	return confirmer.answer, confirmer.err
}
