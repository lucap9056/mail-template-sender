package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/net/html"
)

type TemplateGroups struct {
	dir       string
	templates map[string]*template.Template
}

func New(dirPath string) (*TemplateGroups, error) {

	groups := &TemplateGroups{
		dir:       dirPath,
		templates: make(map[string]*template.Template),
	}

	dir, err := os.ReadDir(dirPath)

	if err != nil {
		return nil, err
	}

	for _, entry := range dir {
		if !entry.IsDir() {
			continue
		}

		if err := groups.readTemplates(entry.Name()); err != nil {
			return nil, err
		}

	}

	return groups, nil
}

func (groups *TemplateGroups) readTemplates(name string) error {

	dirPath := filepath.Join(groups.dir, name)

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	filePaths := []string{}

	for _, file := range files {

		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		filePaths = append(filePaths, filePath)

	}

	templates, err := template.ParseFiles(filePaths...)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %v", err)
	}

	groups.templates[name] = templates

	return nil
}

func (groups *TemplateGroups) ToText(group string, name string, from string, to []string, data any) ([]byte, error) {

	templates, exists := groups.templates[group]
	if !exists {
		return nil, fmt.Errorf("")
	}

	template := templates.Lookup(name)

	if template == nil {

		if template = templates.Lookup("default"); template != nil {
			name = "default"
		} else {
			return nil, fmt.Errorf("template not found: %s", name)
		}

	}

	var body bytes.Buffer

	err := template.Execute(&body, data)
	if err != nil {
		return nil, fmt.Errorf("template execution error: %v", err)
	}

	content := bytes.TrimSpace(body.Bytes())

	title, err := extractTitle(&body)
	if err != nil {
		return nil, fmt.Errorf("template %s title not found: %v", name, err)
	}

	head := fmt.Sprintf(`From: %s
To: %s
Subject: %s
MIME-version: 1.0;
Content-Type: text/html; charset="UTF-8";


`, from, strings.Join(to, ", "), title)

	return append([]byte(head), content...), nil

}

func extractTitle(body *bytes.Buffer) (string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %v", err)
	}

	var title string
	var findTitle func(*html.Node) bool
	findTitle = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findTitle(c) {
				return true
			}
		}
		return false
	}

	if !findTitle(doc) {
		return "", fmt.Errorf("no <title> tag found")
	}

	return title, nil
}
