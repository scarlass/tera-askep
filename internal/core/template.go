package core

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"text/template"
)

func CreateMarker() string {
	s := rand.Text()
	return fmt.Sprintf("<!-- %s -->", s)
}
func ReplaceTemplateString(source string, data map[string]any) (string, error) {
	buf := bytes.NewBufferString("")

	tpl := template.New("replacer")
	tpl.Parse(source)

	if err := tpl.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
