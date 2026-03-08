package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"goose-go/internal/app"
	"goose-go/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, app.ErrInterrupted) {
			os.Exit(130)
		}
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
		sessionID := fs.String("session", "", "resume an existing session by id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		prompt := strings.Join(fs.Args(), " ")
		ctx, cancel := app.RunAgentContext()
		defer cancel()
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		return app.RunAgent(ctx, os.Stdin, os.Stdout, prompt, app.RunOptions{
			Approve:       *approve,
			DebugProvider: *debugProvider,
			SessionID:     *sessionID,
		})
	case "sessions":
		fs := flag.NewFlagSet("sessions", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ctx, cancel := app.RunAgentContext()
		defer cancel()
		return app.ListSessions(ctx, os.Stdout, app.RunOptions{})
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
	case "tui":
		fs := flag.NewFlagSet("tui", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sessionID := fs.String("session", "", "resume an existing session by id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ctx, cancel := app.RunAgentContext()
		defer cancel()
		runtime, err := app.OpenRuntime(os.Stdin, os.Stdout, app.RunOptions{})
		if err != nil {
			return err
		}
		defer func() { _ = runtime.Close() }()
		return tui.Run(ctx, os.Stdin, os.Stdout, runtime, tui.Options{
			SessionID: *sessionID,
		})
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
