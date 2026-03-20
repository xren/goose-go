package app

import (
	"context"
	"io"

	"goose-go/internal/auth/codex"
)

func LoginCodex(ctx context.Context, in io.Reader, out io.Writer) error {
	return codex.Login(ctx, in, out)
}
