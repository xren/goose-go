package main

import (
	"fmt"
	"os"

	"goose-go/internal/repocheck"
)

func main() {
	issues, err := repocheck.Check()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(issues) == 0 {
		return
	}

	fmt.Fprintln(os.Stderr, "repository hygiene violations:")
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "- %s\n", issue)
	}
	os.Exit(1)
}
