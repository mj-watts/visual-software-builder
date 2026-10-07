package studio

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Database  string
	Roots     []string
	Artifacts string
	Timeout   time.Duration
	DemoDelay time.Duration
	Binaries  map[string]string
	Keys      map[string]string
}

func (c *Config) defaults() {
	root, _ := filepath.Abs("..")
	if c.Database == "" {
		c.Database = "swimlane.sqlite3"
	}
	if len(c.Roots) == 0 {
		c.Roots = []string{root}
	}
	if c.Artifacts == "" {
		c.Artifacts = filepath.Join(root, "backend-go", ".swimlane")
	}
	if c.Timeout == 0 {
		c.Timeout = 600 * time.Second
	}
	if c.DemoDelay == 0 {
		c.DemoDelay = 3 * time.Second
	}
	if c.Binaries == nil {
		c.Binaries = map[string]string{}
	}
	if c.Keys == nil {
		c.Keys = map[string]string{}
	}
}
func ConfigFromEnv() Config {
	c := Config{Database: os.Getenv("SWIMLANE_DB"), Artifacts: os.Getenv("SWIMLANE_ARTIFACTS"), Binaries: map[string]string{}, Keys: map[string]string{}}
	if roots := os.Getenv("SWIMLANE_WORKSPACE_ROOTS"); roots != "" {
		c.Roots = filepath.SplitList(roots)
	}
	seconds, err := strconv.ParseFloat(os.Getenv("SWIMLANE_RUN_TIMEOUT"), 64)
	if err == nil {
		c.Timeout = time.Duration(max(1, min(3600, seconds)) * float64(time.Second))
	}
	c.Keys["codex"] = os.Getenv("CODEX_API_KEY")
	if c.Keys["codex"] == "" {
		c.Keys["codex"] = os.Getenv("OPENAI_API_KEY")
	}
	c.Keys["claude"] = os.Getenv("ANTHROPIC_API_KEY")
	for _, p := range []string{"codex", "claude"} {
		if path, err := exec.LookPath(p); err == nil {
			c.Binaries[p] = path
		}
	}
	c.defaults()
	return c
}
func (c Config) enabled(p string) bool { return c.Keys[p] != "" && c.Binaries[p] != "" }
func (c Config) providers() []map[string]any {
	out := []map[string]any{{"id": "demo", "name": "Demo agent", "available": true, "reason": "Simulated execution"}}
	for _, p := range []string{"codex", "claude"} {
		name := map[string]string{"codex": "Codex", "claude": "Claude"}[p]
		key := map[string]string{"codex": "CODEX_API_KEY or OPENAI_API_KEY", "claude": "ANTHROPIC_API_KEY"}[p]
		reason := "Set " + key + " on the backend and install " + p + " CLI"
		if c.enabled(p) {
			reason = "Configured · authentication checked during run"
		}
		out = append(out, map[string]any{"id": p, "name": name, "available": c.enabled(p), "reason": reason})
	}
	return out
}
func LoadEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		loadEnvLine(scanner.Text())
	}
	return scanner.Err()
}
func loadEnvLine(line string) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") {
		return
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok || !validEnvKey(key) {
		return
	}
	if _, set := os.LookupEnv(key); !set {
		_ = os.Setenv(key, strings.Trim(strings.TrimSpace(value), "\"'"))
	}
}

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validEnvKey(key string) bool { return envKeyPattern.MatchString(key) }
