package resource

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed all:*.template
var tmpls embed.FS

var Tmpl *FileTemplate

func init() {
	tpl := template.Must(template.ParseFS(tmpls, "*.template"))
	// tpl.Option("missingkey=invalid")
	Tmpl = &FileTemplate{tpl}
}

type FileTemplate struct {
	tmpl *template.Template
}

func (ft *FileTemplate) Get(name string, data map[string]any) ([]byte, error) {
	buf := bytes.NewBufferString("")
	if err := ft.tmpl.ExecuteTemplate(buf, name, data); err != nil {
		return nil, fmt.Errorf("parse file template: %w", err)
	}
	return buf.Bytes(), nil
}
