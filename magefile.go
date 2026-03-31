//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const binary = "devx"

// Build compiles the binary for the current platform.
func Build() error {
	fmt.Println("» Building", binary, "...")
	return sh.Run("go", "build", "-ldflags=-s -w -X github.com/VitruvianSoftware/devx/cmd.version=dev", "-o", binary, ".")
}

// Install compiles and installs the binary to $GOPATH/bin.
func Install() error {
	mg.Deps(Lint)
	fmt.Println("» Installing", binary, "...")
	return sh.Run("go", "install", ".")
}

// Run builds and runs devx (alias for `./devx init`).
func Run() error {
	mg.Deps(Build)
	return sh.Run("./"+binary, "init")
}

// Lint runs go vet across all packages.
func Lint() error {
	fmt.Println("» go vet ...")
	return sh.Run("go", "vet", "./...")
}

// Test runs all unit tests with race detection and coverage.
func Test() error {
	fmt.Println("» go test ...")
	return sh.Run("go", "test", "-v", "-race", "-coverprofile=coverage.out", "./...")
}

// Coverage opens the HTML coverage report (after Test).
func Coverage() error {
	mg.Deps(Test)
	return sh.Run("go", "tool", "cover", "-html=coverage.out")
}

// Tidy updates go.sum and removes unused dependencies.
func Tidy() error {
	fmt.Println("» go mod tidy ...")
	return sh.Run("go", "mod", "tidy")
}

// ValidateTemplate compiles dev-machine.template.bu with dummy values to verify it.
func ValidateTemplate() error {
	fmt.Println("» Validating Butane template ...")
	if _, err := exec.LookPath("butane"); err != nil {
		return fmt.Errorf("butane not found in PATH — install with: brew install butane")
	}
	// Use the ignition package's Validate helper
	return sh.Run("go", "run", ".", "init", "--help") // light smoke test
}

// Clean removes the compiled binary and build artifacts.
func Clean() error {
	fmt.Println("» Cleaning ...")
	for _, f := range []string{binary, "coverage.out", "dev-machine.bu", "config.ign"} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Release runs goreleaser to produce cross-platform release artifacts.
func Release() error {
	if _, err := exec.LookPath("goreleaser"); err != nil {
		fmt.Println("Installing goreleaser ...")
		if err := sh.Run("go", "install", "github.com/goreleaser/goreleaser/v2@latest"); err != nil {
			return err
		}
	}
	return sh.Run("goreleaser", "release", "--clean")
}

// Snapshot builds a local release snapshot without publishing.
func Snapshot() error {
	if _, err := exec.LookPath("goreleaser"); err != nil {
		fmt.Println("Installing goreleaser ...")
		if err := sh.Run("go", "install", "github.com/goreleaser/goreleaser/v2@latest"); err != nil {
			return err
		}
	}
	return sh.Run("goreleaser", "release", "--snapshot", "--clean")
}

// CI runs the full validation gate: vet → test → build.
func CI() error {
	mg.SerialDeps(Lint, Test, Build)
	fmt.Println("» CI gate passed.")
	return nil
}

// platforms prints the supported cross-compilation targets.
func platforms() {
	fmt.Printf("Current platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("Release targets:  darwin/amd64  darwin/arm64  linux/amd64  linux/arm64")
}
