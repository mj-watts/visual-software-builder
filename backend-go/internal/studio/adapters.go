package studio

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func promptFor(d Draft) string {
	return "Work only in this isolated working directory. Do not commit, push or merge. Follow the repository instructions. Do not start background services. Summarize your changes and any limitations. Tests will be run separately by the host if enabled.\n\n" + d.Prompt
}
func (c Config) redact(text string) string {
	for _, key := range c.Keys {
		if key != "" {
			text = strings.ReplaceAll(text, key, "[redacted]")
		}
	}
	return text
}
func childEnvironment(c Config, provider string, run Run) ([]string, error) {
	env := []string{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	if provider == "codex" {
		home := filepath.Join(filepath.Dir(run.Worktree), "codex-home")
		if err := os.MkdirAll(home, 0700); err != nil {
			return nil, err
		}
		env = append(env, "CODEX_HOME="+home, "CODEX_API_KEY="+c.Keys[provider])
	}
	if provider == "claude" {
		env = append(env, "ANTHROPIC_API_KEY="+c.Keys[provider])
	}
	return env, nil
}
func writeImages(run Run) ([]string, error) {
	dir := filepath.Join(filepath.Dir(run.Worktree), "attachments")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	paths := []string{}
	for i, image := range run.Snapshot.Images {
		header, encoded, _ := strings.Cut(image, ",")
		ext := map[string]string{"data:image/png;base64": "png", "data:image/jpeg;base64": "jpg", "data:image/webp;base64": "webp"}[header]
		path := filepath.Join(dir, fmt.Sprintf("reference-%d.%s", i, ext))
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
func codexCommand(c Config, run Run) ([]string, string, error) {
	d := run.Snapshot
	argv := []string{c.Binaries["codex"], "-a", "never", "exec", "--ignore-user-config", "--ignore-rules", "--sandbox", d.Permission, "-c", `shell_environment_policy.exclude=["CODEX_API_KEY","OPENAI_API_KEY","ANTHROPIC_API_KEY"]`, "--ephemeral", "--json", "--color", "never", "-C", run.Worktree}
	images, err := writeImages(run)
	if err != nil {
		return nil, "", err
	}
	for _, path := range images {
		argv = append(argv, "--image", path)
	}
	return append(argv, "-"), promptFor(d), nil
}
func claudeCommand(c Config, run Run) ([]string, string, error) {
	tools := "Read,Glob,Grep"
	if run.Snapshot.Permission == "workspace-write" {
		tools += ",Edit,Write"
	}
	argv := []string{c.Binaries["claude"], "--print", "--bare", "--restricted", "--disable-slash-commands", "--no-session-persistence", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--permission-mode", "dontAsk", "--tools", tools, "--allowedTools", tools, "--input-format", "stream-json", "--output-format", "stream-json", "--verbose"}
	content := []any{map[string]any{"type": "text", "text": promptFor(run.Snapshot)}}
	for _, image := range run.Snapshot.Images {
		header, data, _ := strings.Cut(image, ",")
		media := strings.Split(strings.TrimPrefix(header, "data:"), ";")[0]
		content = append(content, map[string]any{"type": "image", "source": map[string]string{"type": "base64", "media_type": media, "data": data}})
	}
	data, err := json.Marshal(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}})
	return argv, string(data) + "\n", err
}

type agentResult struct {
	response  string
	completed bool
	failure   string
}

func cliAgent(ctx context.Context, c Config, run Run, emit emitFunc) (string, error) {
	builder := codexCommand
	if run.Snapshot.Provider == "claude" {
		builder = claudeCommand
	}
	argv, input, err := builder(c, run)
	if err != nil {
		return "", err
	}
	env, err := childEnvironment(c, run.Snapshot.Provider, run)
	if err != nil {
		return "", err
	}
	result := agentResult{}
	consume := func(kind, line string) error {
		return consumeEvent(c, run.Snapshot.Provider, &result, kind, line, emit)
	}
	if err = runProcess(ctx, argv, run.Worktree, input, env, consume); err != nil {
		return "", err
	}
	if result.failure != "" {
		return "", fmt.Errorf("%s", result.failure)
	}
	if !result.completed {
		return "", fmt.Errorf("Provider ended without a successful completion event.")
	}
	if result.response == "" {
		result.response = "Agent completed without a text summary. Review the actual Git diff."
	}
	return result.response, nil
}
func consumeEvent(c Config, provider string, result *agentResult, kind, line string, emit emitFunc) error {
	if kind == "stderr" {
		return emit("diagnostic", c.redact(line))
	}
	var data map[string]any
	if json.Unmarshal([]byte(line), &data) != nil {
		return emit("diagnostic", c.redact(line))
	}
	var messages []string
	if provider == "claude" {
		messages = claudeEvents(data, result)
	} else {
		messages = codexEvents(data, result)
	}
	result.response = c.redact(result.response)
	for _, message := range messages {
		if err := emit("agent", c.redact(message)); err != nil {
			return err
		}
	}
	return nil
}
func str(m map[string]any, key string) string         { v, _ := m[key].(string); return v }
func obj(m map[string]any, key string) map[string]any { v, _ := m[key].(map[string]any); return v }
func codexEvents(data map[string]any, result *agentResult) []string {
	kind := str(data, "type")
	if kind == "turn.completed" {
		result.completed = true
	}
	if kind == "turn.failed" || kind == "error" {
		result.failure = "Codex reported an unsuccessful run. See activity for details."
	}
	item := obj(data, "item")
	if str(item, "type") == "agent_message" {
		result.response = str(item, "text")
		return []string{result.response}
	}
	return nil
}
func claudeEvents(data map[string]any, result *agentResult) []string {
	if str(data, "type") == "result" {
		result.completed = true
		result.response = str(data, "result")
		denials, _ := data["permission_denials"].([]any)
		if data["is_error"] == true || len(denials) > 0 {
			result.failure = "Claude reported an error or a denied tool permission. See activity for details."
		}
		return []string{result.response}
	}
	content, _ := obj(data, "message")["content"].([]any)
	return textContent(content)
}
func textContent(content []any) []string {
	messages := []string{}
	for _, item := range content {
		value, ok := item.(map[string]any)
		if ok && str(value, "type") == "text" {
			messages = append(messages, str(value, "text"))
		}
	}
	return messages
}
