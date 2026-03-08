package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"goose-go/internal/models"
)

func ListModels(ctx context.Context, out io.Writer, opts RunOptions) error {
	providerID := strings.TrimSpace(opts.Provider)
	if providerID == "" {
		providerID = defaultProviderName
	}
	resolver := models.NewResolver()
	available, err := resolver.ListAvailable(ctx, providerID)
	if err != nil {
		return err
	}
	for _, item := range available {
		status := "available"
		if !item.Available {
			status = "unavailable"
		}
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s", item.Model.ID, item.Model.DisplayName, status); err != nil {
			return err
		}
		if item.Reason != "" {
			if _, err := fmt.Fprintf(out, "\t%s", item.Reason); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
