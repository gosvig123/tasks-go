package gist

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const gistGitRemote = "https://gist.github.com/%s.git"

func (c *Client) syncWithGit(files map[string]GistFile) error {
	dir, err := os.MkdirTemp("", "tasks-gist-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	env, err := gitAuthEnv(dir, c.config.Token)
	if err != nil {
		return err
	}

	repo := filepath.Join(dir, "repo")
	url := fmt.Sprintf(gistGitRemote, c.config.GistID)
	if err := runGit(dir, env, "clone", "--quiet", url, repo); err != nil {
		return err
	}
	return commitGistFiles(repo, env, files)
}

func gitAuthEnv(dir, token string) ([]string, error) {
	tokenPath := filepath.Join(dir, "token")
	askpassPath := filepath.Join(dir, "askpass.sh")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return nil, fmt.Errorf("writing git token: %w", err)
	}
	script := gitAskpassScript(tokenPath)
	if err := os.WriteFile(askpassPath, []byte(script), 0700); err != nil {
		return nil, fmt.Errorf("writing git askpass: %w", err)
	}
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return append(env, "GIT_ASKPASS="+askpassPath), nil
}

func gitAskpassScript(tokenPath string) string {
	return fmt.Sprintf(`#!/bin/sh
case "$1" in
	*Username*) printf '%%s\n' 'x-access-token' ;;
	*Password*) cat %s ;;
	*) printf '\n' ;;
esac
`, shellQuote(tokenPath))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func commitGistFiles(repo string, env []string, files map[string]GistFile) error {
	if err := writeGistFiles(repo, files); err != nil {
		return err
	}
	if err := runGit(repo, env, "config", "user.name", "tasks-go sync"); err != nil {
		return err
	}
	if err := runGit(repo, env, "config", "user.email", "tasks-go-sync@users.noreply.github.com"); err != nil {
		return err
	}
	return pushGistFiles(repo, env, sortedFileNames(files))
}

func writeGistFiles(repo string, files map[string]GistFile) error {
	for name, file := range files {
		if strings.ContainsAny(name, `/\`) {
			return fmt.Errorf("unsupported gist filename %q", name)
		}
		path := filepath.Join(repo, name)
		if err := os.WriteFile(path, []byte(file.Content), 0644); err != nil {
			return fmt.Errorf("writing gist file %s: %w", name, err)
		}
	}
	return nil
}

func pushGistFiles(repo string, env []string, names []string) error {
	args := append([]string{"add", "--"}, names...)
	if err := runGit(repo, env, args...); err != nil {
		return err
	}
	changed, err := hasStagedGitChanges(repo, env)
	if err != nil || !changed {
		return err
	}
	message := "Sync tasks " + time.Now().Format("2006-01-02 15:04:05")
	if err := runGit(repo, env, "commit", "-m", message, "--quiet"); err != nil {
		return err
	}
	return runGit(repo, env, "push", "--quiet", "origin", "HEAD")
}

func sortedFileNames(files map[string]GistFile) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hasStagedGitChanges(repo string, env []string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = repo
	cmd.Env = env
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("checking git changes: %w", err)
}

func runGit(dir string, env []string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("git %s: %s", args[0], msg)
}
