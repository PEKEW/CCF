// Command ccfl is the Go core for Claude Code's Feishu-managed session mode.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/PEKEW/CCF/internal/app"
	"github.com/PEKEW/CCF/internal/hooks"
)

const usage = `ccfl - cc-feishu-link core

Usage:
  ccfl init
  ccfl status [--session ID]
  ccfl sync [--session ID] [--force] [--dry-run]
  ccfl hook <event> [--dry-run]
  ccfl mcp

Hook events:
  session-start user-prompt-submit pre-tool-use post-tool-use
  stop pre-compact post-compact session-end

Environment:
  CCFL_HOME     state dir (default ~/.cc-feishu-link)
  CCFL_BACKEND  mock | real (default: real if creds set, else mock)
  CCFL_MOCK_DIR mock backend output dir
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "ccfl: error:", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	switch cmd {
	case "init":
		return cmdInit()
	case "status":
		return cmdStatus(args)
	case "sync":
		return cmdSync(args)
	case "hook":
		return cmdHook(args)
	case "mcp":
		return cmdMCP(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Fprint(os.Stderr, usage)
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func cmdInit() error {
	p, created, err := app.Init()
	if err != nil {
		return err
	}
	if created {
		fmt.Println("created config:", p)
	} else {
		fmt.Println("config already exists:", p)
	}
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	id := fs.String("session", "", "session id (default: latest)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	a, err := app.New(false)
	if err != nil {
		return err
	}
	return a.RunStatus(*id, os.Stdout)
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	id := fs.String("session", "", "session id (default: latest)")
	force := fs.Bool("force", false, "ignore policy thresholds")
	dry := fs.Bool("dry-run", false, "do not call Feishu API")
	if err := fs.Parse(args); err != nil {
		return err
	}
	a, err := app.New(*dry)
	if err != nil {
		return err
	}
	return a.RunSync(*id, *force, os.Stdout)
}

func cmdHook(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("hook requires an event name")
	}
	event := args[0]
	fs := flag.NewFlagSet("hook", flag.ContinueOnError)
	dry := fs.Bool("dry-run", false, "do not call Feishu API")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	in, err := hooks.ParseInput(os.Stdin)
	if err != nil {
		return fmt.Errorf("parse hook input: %w", err)
	}
	a, err := app.New(*dry)
	if err != nil {
		return err
	}

	switch event {
	case "session-start":
		return a.RunSessionStart(in, os.Stdout)
	case "user-prompt-submit":
		return a.RunUserPromptSubmit(in, os.Stdout)
	case "pre-tool-use":
		return a.RunPreToolUse(in, os.Stdout)
	case "post-tool-use":
		return a.RunPostToolUse(in, os.Stdout)
	case "stop":
		return a.RunStop(in, os.Stdout)
	case "pre-compact":
		return a.RunPreCompact(in, os.Stdout)
	case "post-compact":
		return a.RunPostCompact(in, os.Stdout)
	case "session-end":
		return a.RunSessionEnd(in, os.Stdout)
	default:
		return fmt.Errorf("unknown hook event %q", event)
	}
}
