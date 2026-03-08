package main

import (
	"fmt"
	"os"

	"goose-go/internal/archcheck"
)

func main() {
	violations, err := archcheck.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "architecture violations:")
		for _, violation := range violations {
			fmt.Fprintf(os.Stderr, "- %s\n", violation)
		}
		os.Exit(1)
	}
}
