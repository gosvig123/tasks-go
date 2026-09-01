package gist

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitGistFilesPushesCheckedOutBranch(t *testing.T) {
	remote, clone := setupGitSyncRepo(t)
	files := map[string]GistFile{"tasks.md": {Content: "updated\n"}}
	if err := commitGistFiles(clone, os.Environ(), files); err != nil {
		t.Fatal(err)
	}
	content := runTestGit(t, "", "--git-dir", remote, "show", "trunk:tasks.md")
	if content != "updated" {
		t.Fatalf("trunk content = %q, want updated", content)
	}
}

func setupGitSyncRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runTestGit(t, "", "init", "--bare", "--initial-branch=trunk", remote)
	seed := filepath.Join(root, "seed")
	runTestGit(t, "", "clone", remote, seed)
	runTestGit(t, seed, "config", "user.name", "test")
	runTestGit(t, seed, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(seed, "tasks.md"), []byte("initial\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, seed, "add", "tasks.md")
	runTestGit(t, seed, "commit", "-m", "initial")
	runTestGit(t, seed, "push", "origin", "HEAD")
	clone := filepath.Join(root, "clone")
	runTestGit(t, "", "clone", remote, clone)
	return remote, clone
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
