package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jctanner/markov/pkg/parser"
	"gopkg.in/yaml.v3"
)

func (e *Engine) loadArtifacts(artifacts map[string]parser.Artifact, runCtx map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for name, art := range artifacts {
		path, err := e.tmpl.Render(art.Path, runCtx)
		if err != nil {
			return nil, fmt.Errorf("artifact %q: rendering path: %w", name, err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) && art.Optional {
				result[name] = nil
				continue
			}
			return nil, fmt.Errorf("artifact %q: reading %s: %w", name, path, err)
		}

		switch art.Format {
		case "yaml":
			var parsed map[string]any
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				return nil, fmt.Errorf("artifact %q: parsing YAML: %w", name, err)
			}
			result[name] = parsed

		case "markdown":
			frontmatter, content := parseMarkdown(string(data))
			result[name] = map[string]any{
				"frontmatter": frontmatter,
				"content":     content,
			}

		default:
			result[name] = string(data)
		}
	}

	return result, nil
}

func parseMarkdown(raw string) (map[string]any, string) {
	scanner := bufio.NewScanner(strings.NewReader(raw))

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, raw
	}

	var fmLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines = append(fmLines, line)
	}

	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &frontmatter); err != nil {
		return nil, raw
	}

	var contentLines []string
	for scanner.Scan() {
		contentLines = append(contentLines, scanner.Text())
	}

	return frontmatter, strings.Join(contentLines, "\n")
}
