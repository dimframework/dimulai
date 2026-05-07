//go:build ignore

// rename renames this starter kit project to a new module path and package name.
//
// Usage:
//
//	go run rename.go <new-module-path>
//
// Examples:
//
//	go run rename.go github.com/myorg/myapp
//	go run rename.go github.com/acme/backend
//
// The script updates:
//   - go.mod  — module declaration
//   - *.go    — package declarations and import paths
//
// After running, execute: go mod tidy && go build ./...
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	oldModule = "github.com/dimframework/dimulai"
	oldPkg    = "dimulai"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:   go run rename.go <new-module-path>")
		fmt.Fprintln(os.Stderr, "Example: go run rename.go github.com/myorg/myapp")
		os.Exit(1)
	}

	newModule := strings.TrimSpace(os.Args[1])
	if newModule == "" {
		fmt.Fprintln(os.Stderr, "Error: new module path cannot be empty")
		os.Exit(1)
	}

	// Derive package name from the last path segment.
	segments := strings.Split(newModule, "/")
	newPkg := segments[len(segments)-1]

	if err := validateIdentifier(newPkg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid package name %q: %v\n", newPkg, err)
		os.Exit(1)
	}

	if newModule == oldModule {
		fmt.Println("Module path is already", oldModule, "— nothing to do.")
		os.Exit(0)
	}

	fmt.Printf("Module:  %s  →  %s\n", oldModule, newModule)
	if newPkg != oldPkg {
		fmt.Printf("Package: %s  →  %s\n", oldPkg, newPkg)
	}
	fmt.Println()

	updated, skipped, errs := walk(".", oldModule, newModule, oldPkg, newPkg)

	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "  error:", e)
	}

	for _, f := range updated {
		fmt.Println("  updated:", f)
	}

	fmt.Printf("\n✓ %d file(s) updated", len(updated))
	if len(skipped) > 0 {
		fmt.Printf(", %d unchanged", len(skipped))
	}
	if len(errs) > 0 {
		fmt.Printf(", %d error(s)", len(errs))
	}
	fmt.Println(".")

	fmt.Println("\nNext steps:")
	fmt.Println("  go mod tidy")
	fmt.Println("  go build ./...")
}

// walk traverses the project tree and applies replacements to eligible files.
func walk(root, oldMod, newMod, oldPkg, newPkg string) (updated, skipped []string, errs []error) {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}

		// Skip hidden directories (not the "." root itself) and vendor.
		if info.IsDir() {
			name := info.Name()
			if name != "." && (strings.HasPrefix(name, ".") || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}

		if !eligible(path) {
			return nil
		}

		changed, err := replaceInFile(path, oldMod, newMod, oldPkg, newPkg)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		if changed {
			updated = append(updated, path)
		} else {
			skipped = append(skipped, path)
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return
}

// eligible reports whether a file should be processed.
func eligible(path string) bool {
	base := filepath.Base(path)
	// Skip the rename script itself.
	if base == "rename.go" {
		return false
	}
	return strings.HasSuffix(path, ".go") || base == "go.mod"
}

// replaceInFile performs all substitutions in a single file.
// Returns true if the file was modified.
func replaceInFile(path, oldMod, newMod, oldPkg, newPkg string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	src := string(raw)
	dst := src

	// 1. Replace module path (handles imports and go.mod module declaration).
	//    Using oldMod as prefix so sub-packages are also covered:
	//      "github.com/dimframework/dimulai/migrations" → "github.com/myorg/myapp/migrations"
	dst = strings.ReplaceAll(dst, oldMod, newMod)

	// 2. Replace package declaration in .go files (skip go.mod).
	if strings.HasSuffix(path, ".go") && oldPkg != newPkg {
		dst = replacePkgDecl(dst, oldPkg, newPkg)
	}

	if dst == src {
		return false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return true, os.WriteFile(path, []byte(dst), info.Mode())
}

// replacePkgDecl replaces `package <oldPkg>` declarations while avoiding
// false positives in comments, strings, or identifiers that start with oldPkg.
func replacePkgDecl(src, oldPkg, newPkg string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Match: "package dimulai" optionally followed by a comment.
		if strings.HasPrefix(trimmed, "package ") {
			rest := strings.TrimPrefix(trimmed, "package ")
			// Extract the identifier (stop at space or end).
			ident := strings.FieldsFunc(rest, func(r rune) bool {
				return unicode.IsSpace(r)
			})
			if len(ident) > 0 && ident[0] == oldPkg {
				lines[i] = strings.Replace(line, "package "+oldPkg, "package "+newPkg, 1)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// validateIdentifier checks that s is a valid Go package name.
func validateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("empty string")
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("must start with a letter")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("invalid character %q", r)
		}
	}
	return nil
}
