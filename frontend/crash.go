package frontend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// crashPrompter asks the user whether to submit a crash report. It is a native
// OS dialog because a panic has already torn down the ebiten UI loop, so the
// in-app report panel is gone with it.
type crashPrompter interface {
	// confirmCrashReport shows a modal asking whether to submit. available is
	// false on a platform with no native dialog wired, in which case confirmed
	// is ignored and no report is sent.
	confirmCrashReport(message string) (confirmed bool, available bool)
}

// defaultCrashPrompter is the cross-platform fallback: no native dialog, so the
// handler writes the bundle to disk and exits without asking. A platform build
// file (crash_windows.go) replaces it with a real dialog.
type defaultCrashPrompter struct{}

func (defaultCrashPrompter) confirmCrashReport(string) (bool, bool) {
	return false, false
}

// platformCrashPrompter is swapped in by a platform build file's init.
var platformCrashPrompter crashPrompter = defaultCrashPrompter{}

// crashEnvironment gathers the collaborators the crash handler needs so tests
// can stand in for the native dialog, the relay, the browser, and process exit.
type crashEnvironment struct {
	prompter crashPrompter
	relay    issueRelayService
	openURL  func(string) error
	exit     func(int)
}

func defaultCrashEnvironment(shell *Shell) crashEnvironment {
	relay := shell.issueRelay
	if relay == nil {
		relay = newIssueRelayClient()
	}
	return crashEnvironment{
		prompter: platformCrashPrompter,
		relay:    relay,
		openURL:  openExternalURL,
		exit:     os.Exit,
	}
}

// runGameWithCrashReporting runs the ebiten loop and, on an unrecovered panic
// from the main loop, offers the user a one-click crash report before exiting
// non-zero. A clean return (including RunGame's own error) passes straight
// through.
func runGameWithCrashReporting(shell *Shell, run func() error) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}
		handleShellCrash(shell, defaultCrashEnvironment(shell), recovered, debug.Stack())
	}()
	return run()
}

// handleShellCrash builds a crash bundle, offers to submit it, and exits. It is
// the last thing that runs after a main-loop panic, so it must never itself
// panic; every step that touches machine state is guarded.
func handleShellCrash(
	shell *Shell,
	env crashEnvironment,
	recovered any,
	stack []byte,
) {
	panicType := fmt.Sprintf("%T", recovered)
	summary := fmt.Sprintf("%v", recovered)

	path, warning := buildCrashBundle(shell, recovered, stack)
	if path != "" {
		fmt.Fprintf(os.Stderr, "ARAM crash bundle written to %s\n", path)
	}

	confirmed, available := false, false
	if env.prompter != nil {
		confirmed, available = env.prompter.confirmCrashReport(fmt.Sprintf(
			"ARAM crashed (%s).\n\n"+
				"Submit a crash report so it can be fixed?\n\n"+
				"The report is attached to a public issue. Host file paths\n"+
				"are removed. It carries logs, diagnostics, and - when the\n"+
				"emulated machine faulted - a few kilobytes of guest memory\n"+
				"from around the crash.",
			panicType,
		))
	}
	if available && confirmed && path != "" {
		submitCrashReport(shell, env, summary, panicType, path, warning)
	}
	if env.exit != nil {
		env.exit(1)
	}
}

// buildCrashBundle collects a debug bundle for the crash. A machine that just
// panicked can panic again when queried, so the whole collection is recovered:
// a failed collection returns an empty path rather than taking the process down
// a second time.
func buildCrashBundle(
	shell *Shell,
	recovered any,
	stack []byte,
) (path string, warning string) {
	defer func() {
		if again := recover(); again != nil {
			path = ""
			warning = fmt.Sprintf("crash bundle collection failed: %v", again)
		}
	}()
	snapshot := shell.captureCrashSnapshot(recovered, stack)
	bundlePath, bundleWarning, err := collectDebugBundle(snapshot, shell.backend)
	if err != nil {
		return "", err.Error()
	}
	return bundlePath, bundleWarning
}

// captureCrashSnapshot reuses the ordinary bundle snapshot but drops the
// screenshot (the GL context died with the loop) and adds the panic and full
// goroutine stack as crash.txt.
func (s *Shell) captureCrashSnapshot(recovered any, stack []byte) debugBundleSnapshot {
	snapshot := s.captureDebugBundleSnapshot(time.Now().UTC())
	snapshot.Screenshot = nil
	report := fmt.Sprintf("panic: %v\n\n%s", recovered, stack)
	snapshot.CrashReport = []byte(redactDebugText(report, snapshot.Redactions))
	return snapshot
}

// submitCrashReport uploads the bundle through the relay and opens the created
// issue. On a relay failure it falls back to a pre-filled GitHub draft plus the
// bundle folder, exactly as the interactive report path does.
func submitCrashReport(
	shell *Shell,
	env crashEnvironment,
	summary, panicType, path, warning string,
) {
	idempotencyKey, err := newIssueIdempotencyKey()
	if err != nil {
		return
	}
	input := crashInput(shell)
	draft := issueReportDraft{
		Situation: shorten(fmt.Sprintf(
			"Automatic crash report: %s - %s", panicType, summary,
		), 500),
		GameTitle:  crashGameTitle(shell, input),
		Repository: "aram-frontend",
	}
	report, relayErr := env.relay.Submit(context.Background(), issueRelaySubmission{
		Draft:          draft,
		Input:          input,
		Backend:        shell.backendName(),
		State:          shell.state,
		BundlePath:     path,
		Warning:        warning,
		IdempotencyKey: idempotencyKey,
	})
	if relayErr != nil {
		draftURL, buildErr := buildIssueDraftURL(
			draft, input, shell.backendName(), shell.state, path, warning,
		)
		if buildErr == nil && env.openURL != nil {
			_ = env.openURL(draftURL)
			_ = openArtifactFolder(filepath.Dir(path))
		}
		return
	}
	shell.settings.rememberIssueReport(IssueReportRecord{
		ReportID:   report.ReportID,
		IssueURL:   report.IssueURL,
		Capability: report.Capability,
		Repository: draft.Repository,
		Situation:  draft.Situation,
		GameTitle:  draft.GameTitle,
		CreatedAt:  time.Now().UTC(),
	})
	_ = shell.settings.save()
	if env.openURL != nil {
		_ = env.openURL(report.IssueURL)
	}
}

func crashInput(shell *Shell) *InputInfo {
	if shell.input == nil {
		return nil
	}
	value := *shell.input
	return &value
}

func crashGameTitle(shell *Shell, input *InputInfo) string {
	if input != nil && input.DisplayName != "" {
		return input.DisplayName
	}
	return "Unknown title"
}
