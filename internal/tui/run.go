package tui

import (
	"errors"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/dff652/ai-asset-hub/internal/apply"
	"github.com/dff652/ai-asset-hub/internal/inventory"
	"github.com/dff652/ai-asset-hub/internal/workspace"
)

var ErrNotTTY = errors.New("interactive terminal required")

type DeploymentResult struct {
	Diff  apply.Report
	Apply *apply.Report
}

type Options struct {
	Home    string
	Project string
	// Workspace immediately enables Phase B composition. Empty starts read-only;
	// the user may explicitly name a path in the TUI (ADR-0006 §2).
	Workspace string
	// Package enables Phase C diff/apply review. Targets uses the same target
	// ids as the CLI diff/apply commands.
	Package        string
	ExpectedSHA256 string
	Targets        []string
	Input          io.Reader
	Output         io.Writer
}

type terminalFile interface {
	Fd() uintptr
}

// Run starts the workflow UI. It delegates scan, composition, build, diff,
// apply, doctor, and rollback to the same Core functions as the CLI.
func Run(options Options) error {
	return run(options, os.Getenv, term.IsTerminal)
}

// IsInteractive reports whether input and output can safely host an
// interactive workflow. Bootstrap uses it before pull so a non-TTY invocation
// cannot retrieve files and only then discover that confirmation is impossible.
func IsInteractive(input io.Reader, output io.Writer) bool {
	return interactiveTerminal(
		Options{Input: input, Output: output},
		os.Getenv,
		term.IsTerminal,
	)
}

// RunDeployment starts the same Phase C review used by `aiah ui --package` and
// returns the final Core reports after the alternate screen closes.
func RunDeployment(options Options) (DeploymentResult, error) {
	if options.Package == "" {
		return DeploymentResult{}, apply.ErrInvalidOptions
	}
	model, err := runModel(options, os.Getenv, term.IsTerminal, false)
	result := DeploymentResult{Diff: model.diffReport}
	if model.applyResult != nil {
		report := *model.applyResult
		result.Apply = &report
	}
	if err != nil {
		return result, err
	}
	return result, model.deployErr
}

func run(options Options, getenv func(string) string, isTerminal func(uintptr) bool) error {
	_, err := runModel(options, getenv, isTerminal, true)
	return err
}

func runModel(
	options Options,
	getenv func(string) string,
	isTerminal func(uintptr) bool,
	maintenance bool,
) (Model, error) {
	if !interactiveTerminal(options, getenv, isTerminal) {
		return Model{}, ErrNotTTY
	}
	if options.Workspace != "" {
		root, _, err := workspace.PrepareRoot(options.Workspace, options.Home, options.Project)
		if err != nil {
			return Model{}, err
		}
		options.Workspace = root
	}

	model := NewModel(inventory.Options{Home: options.Home, Project: options.Project}).
		WithWorkspace(options.Workspace).
		WithMaintenance(maintenance).
		WithHome(maintenance).
		WithDeployment(apply.Options{
			Package:        options.Package,
			ExpectedSHA256: options.ExpectedSHA256,
			Home:           options.Home,
			Project:        options.Project,
			Targets:        options.Targets,
		})
	program := tea.NewProgram(
		model,
		tea.WithInput(options.Input),
		tea.WithOutput(options.Output),
		tea.WithAltScreen(),
	)
	final, err := program.Run()
	if err != nil {
		return Model{}, err
	}
	result, ok := final.(Model)
	if !ok {
		return Model{}, errors.New("unexpected final TUI model")
	}
	return result, nil
}

func interactiveTerminal(options Options, getenv func(string) string, isTerminal func(uintptr) bool) bool {
	inputFile, inputOK := options.Input.(terminalFile)
	outputFile, outputOK := options.Output.(terminalFile)
	terminalName := getenv("TERM")
	if !inputOK || !outputOK || terminalName == "" || terminalName == "dumb" ||
		!isTerminal(inputFile.Fd()) || !isTerminal(outputFile.Fd()) {
		return false
	}
	return true
}
