package main

import (
	"os"

	"github.com/PEKEW/CCF/internal/app"
	"github.com/PEKEW/CCF/internal/mcp"
)

// cmdMCP runs the stdio MCP server exposing ccfl's Feishu doc-authoring tools.
func cmdMCP(_ []string) error {
	a, err := app.New(false)
	if err != nil {
		return err
	}
	return mcp.NewServer(a).Serve(os.Stdin, os.Stdout)
}
