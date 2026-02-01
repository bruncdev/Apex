package core

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func RenderFS(efs embed.FS, templateRoot string, data any, destRoot string) error {
	info, err := fs.Stat(efs, templateRoot)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return renderFile(efs, templateRoot, destRoot, data)
	}

	return renderDir(efs, templateRoot, destRoot, data)
}

func renderDir(efs embed.FS, srcDir string, destDir string, data any) error {
	entries, err := fs.ReadDir(efs, srcDir)
	if err != nil {
		return err
	}

	if err := mkdirAll(destDir); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := path.Join(srcDir, entry.Name())
		destName := entry.Name()
		if strings.HasPrefix(destName, "dot.") {
			destName = "." + strings.TrimPrefix(destName, "dot.")
		}
		destPath := filepath.Join(destDir, destName)

		if entry.IsDir() {
			if err := renderDir(efs, srcPath, destPath, data); err != nil {
				return err
			}
			continue
		}

		if err := renderFile(efs, srcPath, destPath, data); err != nil {
			return err
		}
	}

	return nil
}

func renderFile(efs embed.FS, srcPath string, destPath string, data any) error {
	content, err := efs.ReadFile(srcPath)
	if err != nil {
		return err
	}

	if strings.HasSuffix(destPath, ".tmpl") {
		destPath = strings.TrimSuffix(destPath, ".tmpl")
		tmpl, err := template.New(filepath.Base(srcPath)).Option("missingkey=error").Parse(string(content))
		if err != nil {
			return err
		}

		if err := mkdirAll(filepath.Dir(destPath)); err != nil {
			return err
		}

		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return wrapPermission(err, destPath)
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			return err
		}
		return nil
	}

	if err := mkdirAll(filepath.Dir(destPath)); err != nil {
		return err
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return wrapPermission(err, destPath)
	}
	defer f.Close()

	if _, err := io.Copy(f, bytes.NewReader(content)); err != nil {
		return err
	}

	return nil
}

func mkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return wrapPermission(err, dir)
	}
	return nil
}

func wrapPermission(err error, target string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied: %s: %w", target, err)
	}
	return err
}
