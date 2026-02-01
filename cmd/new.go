package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bruncdev/apex/core"
	"github.com/bruncdev/apex/internal/templates"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

type ProjectConfig struct {
	Name         string
	Module       string
	Architecture string
	Database     string
	Docker       bool
	UseGorm      bool
}

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new Go project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := askProjectConfig()
			if err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			dest := filepath.Join(cwd, cfg.Name)
			if err := os.MkdirAll(dest, 0o755); err != nil {
				if os.IsPermission(err) {
					return fmt.Errorf("permission denied creating %s: %w", dest, err)
				}
				return err
			}

			templateRoots := templateRootsFor(cfg)
			if len(templateRoots) == 0 {
				return fmt.Errorf("unknown architecture: %s", cfg.Architecture)
			}

			for _, root := range templateRoots {
				if err := core.RenderFS(templates.FS, root, cfg, dest); err != nil {
					return err
				}
			}

			runGoModInitAndTidy(dest, cfg.Module)
			fmt.Fprintf(os.Stdout, "Project created at %s\n", dest)
			return nil
		},
	}

	return cmd
}

func askProjectConfig() (ProjectConfig, error) {
	cfg := ProjectConfig{}

	if err := survey.AskOne(&survey.Input{
		Message: "Project name:",
		Default: "apex-app",
	}, &cfg.Name, survey.WithValidator(survey.Required)); err != nil {
		return cfg, err
	}

	prompts := []*survey.Question{
		{
			Name: "architecture",
			Prompt: &survey.Select{
				Message: "Architecture:",
				Options: []string{"clean", "modular"},
				Default: "clean",
			},
		},
		{
			Name: "module",
			Prompt: &survey.Input{
				Message: "Module path:",
				Default: cfg.Name,
			},
			Validate: survey.Required,
		},
		{
			Name: "database",
			Prompt: &survey.Select{
				Message: "Database:",
				Options: []string{"postgres", "mysql", "sqlite", "none"},
				Default: "postgres",
			},
		},
		{
			Name: "gorm",
			Prompt: &survey.Confirm{
				Message: "Use GORM?",
				Default: true,
			},
		},
		{
			Name: "docker",
			Prompt: &survey.Confirm{
				Message: "Generate Dockerfile?",
				Default: true,
			},
		},
	}

	answers := struct {
		Architecture string `survey:"architecture"`
		Module       string `survey:"module"`
		Database     string `survey:"database"`
		UseGorm      bool   `survey:"gorm"`
		Docker       bool   `survey:"docker"`
	}{}

	if err := survey.Ask(prompts, &answers); err != nil {
		return cfg, err
	}

	cfg.Architecture = answers.Architecture
	cfg.Module = answers.Module
	cfg.Database = answers.Database
	cfg.UseGorm = answers.UseGorm
	cfg.Docker = answers.Docker
	return cfg, nil
}

func templateRootFor(arch string) string {
	switch arch {
	case "clean":
		return "files/clean"
	case "modular":
		return "files/modular"
	default:
		return ""
	}
}

func templateRootsFor(cfg ProjectConfig) []string {
	base := templateRootFor(cfg.Architecture)
	if base == "" {
		return nil
	}

	roots := []string{base}
	if cfg.Docker {
		roots = append(roots, "files/docker")
	}
	return roots
}

func runGoModInitAndTidy(dest string, module string) {
	initCmd := exec.Command("go", "mod", "init", module)
	initCmd.Dir = dest
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: go mod init failed: %v\n", err)
		return
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dest
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: go mod tidy failed: %v\n", err)
	}
}
