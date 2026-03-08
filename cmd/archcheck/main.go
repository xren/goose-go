package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type goListPackage struct {
	ImportPath string
	Imports    []string
}

type rule struct {
	fromPrefix string
	toPrefix   string
	message    string
}

var rules = []rule{
	{
		fromPrefix: "goose-go/internal/provider",
		toPrefix:   "goose-go/internal/storage/",
		message:    "providers must not depend on storage implementations",
	},
	{
		fromPrefix: "goose-go/internal/provider",
		toPrefix:   "goose-go/cmd/",
		message:    "providers must not depend on CLI entrypoints",
	},
	{
		fromPrefix: "goose-go/internal/session",
		toPrefix:   "goose-go/internal/storage/",
		message:    "session contracts must not depend on storage implementations",
	},
	{
		fromPrefix: "goose-go/internal/session",
		toPrefix:   "goose-go/internal/provider",
		message:    "session contracts must not depend on providers",
	},
	{
		fromPrefix: "goose-go/internal/tools",
		toPrefix:   "goose-go/internal/provider/",
		message:    "tools must not depend on provider implementations",
	},
}

func main() {
	packages, err := loadPackages()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var violations []string
	for _, pkg := range packages {
		for _, imported := range pkg.Imports {
			if pkg.ImportPath != imported && strings.HasPrefix(imported, "goose-go/cmd/") && !strings.HasPrefix(pkg.ImportPath, "goose-go/cmd/") {
				violations = append(violations, fmt.Sprintf("%s imports %s: packages must not depend on CLI entrypoints", pkg.ImportPath, imported))
			}

			for _, rule := range rules {
				if !strings.HasPrefix(pkg.ImportPath, rule.fromPrefix) {
					continue
				}
				if strings.HasPrefix(imported, rule.toPrefix) {
					violations = append(violations, fmt.Sprintf("%s imports %s: %s", pkg.ImportPath, imported, rule.message))
				}
			}
		}
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "architecture violations:")
		for _, violation := range violations {
			fmt.Fprintf(os.Stderr, "- %s\n", violation)
		}
		os.Exit(1)
	}
}

func loadPackages() ([]goListPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []goListPackage
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}
