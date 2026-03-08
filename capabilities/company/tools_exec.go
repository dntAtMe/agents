package company

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/dntatme/agents/tool"
)

// allowedCommands is the allow-list of executable commands.
var allowedCommands = map[string]bool{
	"go":        true,
	"npm":       true,
	"node":      true,
	"npx":       true,
	"python":    true,
	"python3":   true,
	"pip":       true,
	"pip3":      true,
	"make":      true,
	"docker":    true,
	"terraform": true,
	"kubectl":   true,
	"helm":      true,
	"ls":        true,
	"find":      true,
	"grep":      true,
	"diff":      true,
	"curl":      true,
	"cat":       true,
	"dir":       true,
}

// shellOperatorPattern matches dangerous shell operators.
var shellOperatorPattern = regexp.MustCompile(`[|;&` + "`" + `]|\$\(|&&|\|\||>>|>`)

// maxOutputBytes is the maximum output size captured from a command.
const maxOutputBytes = 10 * 1024 // 10KB

// RunCommandTool returns a tool that executes a command in the workspace.
func RunCommandTool() tool.Tool {
	return tool.Func("run_command", "Execute a command in the workspace. Only allowed commands can be run (go, npm, node, python, make, docker, etc). Shell operators are not allowed.").
		StringParam("command", "The command to execute (e.g. 'go build ./...', 'npm test'). No shell operators allowed.", true).
		StringParam("working_dir", "Workspace-relative working directory. Defaults to workspace root.", false).
		IntParam("timeout_seconds", "Timeout in seconds (default 30, max 60).", false).
		Handler(func(ctx context.Context, args map[string]any, state map[string]any) (map[string]any, error) {
			command, _ := args["command"].(string)
			workingDir, _ := args["working_dir"].(string)
			timeoutSec := toInt(args["timeout_seconds"])
			if timeoutSec <= 0 {
				timeoutSec = 30
			}
			if timeoutSec > 60 {
				timeoutSec = 60
			}

			root := GetWorkspaceRoot(state)
			agentName := GetCurrentAgent(state)
			round := GetCurrentRound(state)

			// Reject shell operators
			if shellOperatorPattern.MatchString(command) {
				return map[string]any{"error": "shell operators (|, ;, &&, ||, `, $(), >, >>) are not allowed"}, nil
			}

			// Parse the command
			parts := strings.Fields(command)
			if len(parts) == 0 {
				return map[string]any{"error": "empty command"}, nil
			}

			// Check allow-list
			executable := parts[0]
			if !allowedCommands[executable] {
				return map[string]any{"error": fmt.Sprintf("command %q is not in the allow-list. Allowed: go, npm, node, npx, python, pip, make, docker, terraform, kubectl, helm, ls, find, grep, diff, curl, cat, dir", executable)}, nil
			}

			// Resolve working directory
			cmdDir := root
			if workingDir != "" {
				resolved, err := ResolvePath(root, workingDir)
				if err != nil {
					return map[string]any{"error": err.Error()}, nil
				}
				cmdDir = resolved
			}

			// Create command with timeout
			cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, parts[0], parts[1:]...)
			cmd.Dir = cmdDir
			cmd.Env = buildSafeEnv()

			// Capture output
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stdout

			err := cmd.Run()
			timedOut := cmdCtx.Err() == context.DeadlineExceeded

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					exitCode = -1
				}
			}

			// Cap output
			output := stdout.String()
			if len(output) > maxOutputBytes {
				output = output[:maxOutputBytes] + "\n... (output truncated at 10KB)"
			}

			// Log the command
			cmdLog := GetCommandLog(state)
			cmdLog.Add(CommandResult{
				Command:  command,
				Agent:    agentName,
				Round:    round,
				ExitCode: exitCode,
				Output:   output,
				TimedOut: timedOut,
			})

			// Sync command log
			if root != "" {
				_ = SyncCommandLog(root, cmdLog)
			}

			return map[string]any{
				"stdout":    output,
				"exit_code": exitCode,
				"timed_out": timedOut,
			}, nil
		}).
		Build()
}

// buildSafeEnv returns a restricted set of environment variables.
func buildSafeEnv() []string {
	safeKeys := []string{
		"PATH", "HOME", "GOPATH", "GOROOT", "GOBIN",
		"NODE_PATH", "NPM_CONFIG_PREFIX",
		"PYTHONPATH", "VIRTUAL_ENV",
		"TERM", "LANG", "LC_ALL",
		"TMPDIR", "TEMP", "TMP",
		"SystemRoot", "SYSTEMROOT",
		"HOMEDRIVE", "HOMEPATH", "USERPROFILE",
	}

	var env []string
	for _, key := range safeKeys {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env
}
