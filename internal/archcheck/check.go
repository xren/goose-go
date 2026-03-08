package archcheck

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type GoListPackage struct {
	ImportPath string
	Imports    []string
}

type Rule struct {
	FromPrefix string
	ToPrefix   string
	Message    string
}

func Check() ([]string, error) {
	packages, err := loadPackages()
	if err != nil {
		return nil, err
	}
	return FindViolations(packages, Rules), nil
}

func FindViolations(packages []GoListPackage, rules []Rule) []string {
	var violations []string
	for _, pkg := range packages {
		for _, imported := range pkg.Imports {
			if pkg.ImportPath != imported && strings.HasPrefix(imported, "goose-go/cmd/") && !strings.HasPrefix(pkg.ImportPath, "goose-go/cmd/") {
				violations = append(violations, fmt.Sprintf("%s imports %s: packages must not depend on CLI entrypoints", pkg.ImportPath, imported))
			}
			if pkg.ImportPath != imported && strings.HasPrefix(imported, "goose-go/internal/evals") && !strings.HasPrefix(pkg.ImportPath, "goose-go/internal/evals") {
				violations = append(violations, fmt.Sprintf("%s imports %s: production packages must not depend on eval harness packages", pkg.ImportPath, imported))
			}

			for _, rule := range rules {
				if !strings.HasPrefix(pkg.ImportPath, rule.FromPrefix) {
					continue
				}
				if strings.HasPrefix(imported, rule.ToPrefix) {
					violations = append(violations, fmt.Sprintf("%s imports %s: %s", pkg.ImportPath, imported, rule.Message))
				}
			}
		}
	}
	return violations
}

func loadPackages() ([]GoListPackage, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []GoListPackage
	for decoder.More() {
		var pkg GoListPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}
