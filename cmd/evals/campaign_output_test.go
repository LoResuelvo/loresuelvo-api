package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCampaignOutputRequiresIgnoredDirectoryOnlyInsideRepository(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "--quiet", root)
	require.NoError(t, command.Run())
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/private-campaigns/\n"), 0600))
	require.NoError(t, verifyCampaignOutputInRepository(root, filepath.Join(root, "private-campaigns", "run")))
	require.NoError(t, verifyCampaignOutputInRepository(root, filepath.Join(t.TempDir(), "external-run")))
	err := verifyCampaignOutputInRepository(root, filepath.Join(root, "visible-campaigns", "run"))
	require.ErrorContains(t, err, "ignored directory or outside")
	_, err = os.Stat(filepath.Join(root, "visible-campaigns"))
	require.True(t, os.IsNotExist(err))
}

func TestCampaignOutputRejectsFilenameOnlyIgnoreRules(t *testing.T) {
	for _, pattern := range []string{".eval-evidence-check\n", ".*\n"} {
		t.Run(pattern, func(t *testing.T) {
			root := t.TempDir()
			command := exec.Command("git", "init", "--quiet", root)
			require.NoError(t, command.Run())
			require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(pattern), 0600))
			err := verifyCampaignOutputInRepository(root, filepath.Join(root, "visible-campaigns", "run"))
			require.ErrorContains(t, err, "ignored directory or outside")
		})
	}
}

func TestCampaignOutputResolvesExternalSymlinkToVisibleRepositoryParent(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "init", "--quiet", root)
	require.NoError(t, command.Run())
	visible := filepath.Join(root, "visible-campaigns")
	require.NoError(t, os.Mkdir(visible, 0700))
	link := filepath.Join(t.TempDir(), "external-link")
	require.NoError(t, os.Symlink(visible, link))
	err := verifyCampaignOutputInRepository(root, filepath.Join(link, "new-run"))
	require.ErrorContains(t, err, "ignored directory or outside")
	_, err = os.Stat(filepath.Join(visible, "new-run"))
	require.True(t, os.IsNotExist(err))
}
