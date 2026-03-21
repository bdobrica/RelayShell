package devimage

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

//go:embed templates/Dockerfile.dev.tmpl
var dockerfileTemplateSource string

const defaultDerivedBaseImage = "relayshell-codex:latest"

var dockerfileTemplate = template.Must(template.New("Dockerfile.dev.tmpl").Funcs(template.FuncMap{
	"hasLang": hasLanguage,
}).Parse(dockerfileTemplateSource))

type dockerfileTemplateData struct {
	BaseImage string
	Languages []string
}

func RenderDockerfile(stack Stack) (string, error) {
	languages := languagesForStack(stack)
	data := dockerfileTemplateData{
		BaseImage: defaultDerivedBaseImage,
		Languages: languages,
	}

	var out bytes.Buffer
	if err := dockerfileTemplate.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render dockerfile template: %w", err)
	}

	return out.String(), nil
}

func languagesForStack(stack Stack) []string {
	switch stack {
	case StackGo:
		return []string{"go"}
	case StackPython:
		return []string{"python"}
	case StackNode:
		return []string{"node"}
	case StackMixed:
		return []string{"go", "python", "node"}
	case StackUnknown:
		fallthrough
	default:
		return nil
	}
}

func hasLanguage(languages []string, language string) bool {
	return slices.Contains(languages, strings.ToLower(strings.TrimSpace(language)))
}
