package cli

import (
	"context"
	"flag"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/util"
)

func DoctorCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var provider string
	fs.StringVar(&provider, "provider", "claude-code", "provider id")
	if err := fs.Parse(args); err != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error()))
		return 2
	}

	command, args := providerCommand(provider)
	path, err := util.LookPath(command)
	if err != nil {
		_ = WriteJSON(stdout, map[string]any{
			"provider":           provider,
			"provider_available": false,
			"detail":             err.Error(),
		})
		return 1
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmdArgs := append(args, "--version")
	out, runErr := exec.CommandContext(ctx, path, cmdArgs...).CombinedOutput()
	detail := strings.TrimSpace(string(out))
	if runErr != nil && detail == "" {
		detail = runErr.Error()
	}

	_ = WriteJSON(stdout, map[string]any{
		"provider":           provider,
		"provider_available": true,
		"provider_path":      path,
		"provider_args":      args,
		"detail":             detail,
		"warnings": []string{
			"Clank is designed for local PTY control, not hosted brokering of consumer Claude accounts.",
		},
	})
	return 0
}

func providerCommand(provider string) (string, []string) {
	switch provider {
	case "", "claude-code":
		return claudecode.ResolveCommand(nil)
	default:
		return provider, nil
	}
}
