package studio

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

func trim(b []byte) string { return strings.TrimSpace(string(b)) }
func git(path string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	argv := append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-C", path}, args...)
	b, err := exec.CommandContext(ctx, "git", argv...).Output()
	if err != nil {
		return nil, fmt.Errorf("Git operation failed. Check the repository and saved HEAD: %w", err)
	}
	return b, nil
}
func canonical(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
func within(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func allowedRoot(path string, roots []string) bool {
	for _, root := range roots {
		r, err := canonical(root)
		if err == nil && within(path, r) {
			return true
		}
	}
	return false
}
func relativeDirectory(root, dir string) (string, error) {
	if filepath.IsAbs(dir) {
		return "", fmt.Errorf("Test directory must be relative to the repository.")
	}
	target, err := canonical(filepath.Join(root, dir))
	if err != nil {
		return "", fmt.Errorf("Test directory does not exist.")
	}
	base, err := canonical(root)
	if err != nil {
		return "", err
	}
	if !within(target, base) {
		return "", fmt.Errorf("Test directory must stay inside the worktree.")
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Test directory does not exist.")
	}
	return target, nil
}
func resolveWorkspace(w Workspace, c Config) (Workspace, error) {
	path, err := canonical(w.Path)
	if err != nil {
		return w, err
	}
	if !allowedRoot(path, c.Roots) {
		return w, fmt.Errorf("Repository is outside the configured workspace roots.")
	}
	top, err := git(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return w, err
	}
	resolved, err := canonical(trim(top))
	if err != nil {
		return w, err
	}
	if resolved != path {
		return w, fmt.Errorf("Select the root of a Git repository.")
	}
	if _, err = git(path, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return w, err
	}
	if _, err = relativeDirectory(path, w.TestDirectory); err != nil {
		return w, err
	}
	w.Path = path
	hash := sha256.Sum256([]byte(path))
	w.ID = fmt.Sprintf("ws-%x", hash[:8])
	return w, nil
}
func (a *App) registerWorkspace(r *http.Request) (any, error) {
	w := Workspace{TestPreset: "none", TestDirectory: "."}
	if err := decode(r, &w); err != nil {
		return nil, err
	}
	if err := validateWorkspace(w); err != nil {
		return nil, err
	}
	w, err := resolveWorkspace(w, a.config)
	if err != nil {
		return nil, problem(422, err.Error())
	}
	return w, a.store.saveWorkspace(w)
}
func validateWorkspace(w Workspace) error {
	if err := validateText(w.Name, 1, 120); err != nil {
		return err
	}
	if !textLength(w.Path, 1, 4096) || !textLength(w.TestDirectory, 0, 500) {
		return problem(422, "Invalid repository path or test directory")
	}
	if !slices.Contains([]string{"none", "vitest", "pytest"}, w.TestPreset) {
		return problem(422, "Invalid test preset")
	}
	return nil
}
func prepareWorktree(repo, target, base string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	_, err := git(repo, "worktree", "add", "--detach", target, base)
	return err
}
func collectDiff(path, base string) ([]ChangedFile, error) {
	untracked, err := git(path, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	for _, name := range strings.Split(string(untracked), "\x00") {
		if name == "" {
			continue
		}
		if _, err = git(path, "add", "--intent-to-add", "--", name); err != nil {
			return nil, err
		}
	}
	raw, err := git(path, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--numstat", "-z", base)
	if err != nil {
		return nil, err
	}
	entries := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if len(entries) > 200 {
		return nil, fmt.Errorf("More than 200 changed files. Worktree retained; narrow the ticket before retrying.")
	}
	return readDiffEntries(path, base, entries)
}
func readDiffEntries(path, base string, entries []string) ([]ChangedFile, error) {
	files := []ChangedFile{}
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		file, err := fileDiff(path, base, entry)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}
func fileDiff(path, base, entry string) (ChangedFile, error) {
	parts := strings.SplitN(entry, "\t", 3)
	if len(parts) != 3 {
		return ChangedFile{}, fmt.Errorf("Invalid Git numstat output")
	}
	content, err := git(path, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", base, "--", parts[2])
	if err != nil {
		return ChangedFile{}, err
	}
	if len(content) > 500000 {
		return ChangedFile{}, fmt.Errorf("A file diff exceeds 500 KB. Worktree retained for manual review.")
	}
	added, _ := strconv.Atoi(parts[0])
	removed, _ := strconv.Atoi(parts[1])
	return ChangedFile{Path: parts[2], Additions: added, Deletions: removed, Diff: string(content)}, nil
}
