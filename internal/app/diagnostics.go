package app

import (
	"errors"
	"fmt"
	"strings"
)

type DiagnosticCategory string

const (
	DiagnosticAuthMissing       DiagnosticCategory = "auth_missing"
	DiagnosticAuthInvalid       DiagnosticCategory = "auth_invalid"
	DiagnosticAuthRefreshFailed DiagnosticCategory = "auth_refresh_failed"
	DiagnosticProviderRequest   DiagnosticCategory = "provider_request_failed"
	DiagnosticProviderHTTP      DiagnosticCategory = "provider_http_error"
	DiagnosticProviderStream    DiagnosticCategory = "provider_stream_error"
	DiagnosticProviderEmpty     DiagnosticCategory = "provider_empty_response"
	DiagnosticInterrupted       DiagnosticCategory = "interrupted"
	DiagnosticUnknown           DiagnosticCategory = "unknown"
)

type DiagnosticError struct {
	Category DiagnosticCategory
	Provider string
	Summary  string
	Hint     string
	Cause    error
}

func (e *DiagnosticError) Error() string {
	if e == nil {
		return ""
	}
	if e.Hint == "" {
		return fmt.Sprintf("%s: %s", e.Category, e.Summary)
	}
	return fmt.Sprintf("%s: %s. %s", e.Category, e.Summary, e.Hint)
}

func (e *DiagnosticError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func diagnoseProviderError(providerName string, err error, debug bool) error {
	if err == nil {
		return nil
	}

	diag := classifyProviderError(providerName, err)
	if !debug || diag.Cause == nil {
		return diag
	}
	return fmt.Errorf("%s: %w", diag.Error(), diag.Cause)
}

func diagnoseRunError(providerName string, err error, debug bool) error {
	if err == nil {
		return nil
	}

	diag := classifyProviderError(providerName, err)
	if diag.Category == DiagnosticUnknown {
		diag.Summary = "run failed"
	}
	if !debug || diag.Cause == nil {
		return diag
	}
	return fmt.Errorf("%s: %w", diag.Error(), diag.Cause)
}

func classifyProviderError(providerName string, err error) *DiagnosticError {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInterrupted) {
		return &DiagnosticError{
			Category: DiagnosticInterrupted,
			Provider: providerName,
			Summary:  "run interrupted",
			Cause:    err,
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "auth file not found"):
		return &DiagnosticError{
			Category: DiagnosticAuthMissing,
			Provider: providerName,
			Summary:  "credentials were not found",
			Hint:     "run `codex login`",
			Cause:    err,
		}
	case strings.Contains(msg, "decode codex auth file"),
		strings.Contains(msg, "missing access or refresh token"),
		strings.Contains(msg, "missing account_id"),
		strings.Contains(msg, "auth mode"):
		return &DiagnosticError{
			Category: DiagnosticAuthInvalid,
			Provider: providerName,
			Summary:  "credentials are invalid or unreadable",
			Hint:     "re-run `codex login`",
			Cause:    err,
		}
	case strings.Contains(msg, "refresh codex token"),
		strings.Contains(msg, "decode refresh response"):
		return &DiagnosticError{
			Category: DiagnosticAuthRefreshFailed,
			Provider: providerName,
			Summary:  "credential refresh failed",
			Hint:     "re-run `codex login`",
			Cause:    err,
		}
	case strings.Contains(msg, "codex request failed: status"):
		return &DiagnosticError{
			Category: DiagnosticProviderHTTP,
			Provider: providerName,
			Summary:  "backend rejected the request",
			Hint:     "retry once, then inspect debug output if it repeats",
			Cause:    err,
		}
	case strings.Contains(msg, "send codex request"):
		return &DiagnosticError{
			Category: DiagnosticProviderRequest,
			Provider: providerName,
			Summary:  "request could not be sent",
			Hint:     "check network connectivity and retry",
			Cause:    err,
		}
	case strings.Contains(msg, "provider smoke did not produce a complete response"),
		strings.Contains(msg, "did not produce a final assistant message"):
		return &DiagnosticError{
			Category: DiagnosticProviderEmpty,
			Provider: providerName,
			Summary:  "provider returned no complete assistant response",
			Hint:     "retry with --debug to inspect the stream",
			Cause:    err,
		}
	case strings.Contains(msg, "stream"),
		strings.Contains(msg, "sse"):
		return &DiagnosticError{
			Category: DiagnosticProviderStream,
			Provider: providerName,
			Summary:  "stream processing failed",
			Hint:     "retry with --debug to inspect the stream",
			Cause:    err,
		}
	default:
		return &DiagnosticError{
			Category: DiagnosticUnknown,
			Provider: providerName,
			Summary:  "provider smoke failed",
			Hint:     "retry with --debug for more detail",
			Cause:    err,
		}
	}
}
