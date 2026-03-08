package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"goose-go/internal/app"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Println("goose-go is ready")
		return nil
	}

	switch args[0] {
	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		approve := fs.Bool("approve", false, "prompt before each tool execution")
		debugProvider := fs.Bool("debug-provider", false, "print translated provider request and raw SSE events")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		prompt := strings.Join(fs.Args(), " ")
		ctx, cancel := app.RunAgentContext()
		defer cancel()
		return app.RunAgent(ctx, os.Stdin, os.Stdout, prompt, app.RunOptions{
			Approve:       *approve,
			DebugProvider: *debugProvider,
		})
	case "provider-smoke":
		fs := flag.NewFlagSet("provider-smoke", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		debug := fs.Bool("debug", false, "print translated request, redacted headers, raw SSE events, and normalized events")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		prompt := strings.Join(fs.Args(), " ")

		ctx, cancel := app.ProviderSmokeContext()
		defer cancel()

		return app.RunProviderSmoke(ctx, os.Stdout, prompt, app.ProviderSmokeOptions{
			Debug: *debug,
		})
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
