package cmd

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"text/template"
)

const mainContentTemplate = `
package main

import (
	"github.com/soryetong/gooze-starter/gooze"

	{{if .ServerPath}}_ "{{.ServerPath}}" {{end}}
)

func main() {
	gooze.Run()
}
`

func genMain(targetPath, serverPath string) error {
	tmpl, err := template.New("main").Parse(mainContentTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	var tmplExecErr error
	if serverPath != "" {
		tmplExecErr = tmpl.Execute(&buf, map[string]string{"ServerPath": serverPath})
	} else {
		tmplExecErr = tmpl.Execute(&buf, nil)
	}
	if tmplExecErr != nil {
		return err
	}

	filename := filepath.Join(targetPath, "main.go")
	err = WriteFileWithDirs(filename, buf.Bytes())
	// err = os.WriteFile(filename, buf.Bytes(), 0644)
	if err != nil {
		return err
	}

	return formatFileWithGofmt(filename)
}

func formatFileWithGofmt(file string) error {
	cmd := exec.Command("gofmt", "-w", file)
	return cmd.Run()
}
