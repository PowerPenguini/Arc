package main

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

func renderStringTemplate(name, tpl string, data any) (string, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", name, err)
	}
	return out.String(), nil
}

func renderTemplateFile(path string, data any) (string, error) {
	raw, err := templateFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %q: %w", path, err)
	}
	return renderStringTemplate(path, string(raw), data)
}

func mustTemplateFile(path string) string {
	raw, err := templateFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("read template %q: %v", path, err))
	}
	return string(raw)
}
