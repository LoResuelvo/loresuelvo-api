package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// verifyCampaignOutput prevents generated evidence from invalidating the
// clean-source check at the next phase, before any preflight provider call.
func verifyCampaignOutput(output string) error {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("locate campaign source repository: %w", err)
	}
	return verifyCampaignOutputInRepository(strings.TrimSpace(string(root)), output)
}
func verifyCampaignOutputInRepository(repository, output string) error {
	absolute, err := resolveOutputPath(output)
	if err != nil {
		return fmt.Errorf("resolve campaign output: %w", err)
	}
	relative, err := filepath.Rel(repository, absolute)
	if err != nil {
		return fmt.Errorf("resolve output against repository: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	// A trailing separator applies directory/descendant ignore semantics even
	// before creation; a filename-only or dotfile rule is not sufficient.
	probe := absolute + string(filepath.Separator)
	command := exec.Command("git", "check-ignore", "--quiet", "--no-index", "--", probe)
	command.Dir = repository
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return fmt.Errorf("campaign output inside the source repository must be ignored; use an ignored directory or outside the repository")
		}
		return fmt.Errorf("verify campaign output ignore policy: %w", err)
	}
	return nil
}
