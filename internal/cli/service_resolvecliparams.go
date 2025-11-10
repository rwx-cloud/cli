package cli

import (
	"os"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/rwx-cloud/cli/internal/errors"
)

var gitInitParams = map[string]bool{
	"sha":    true,
	"ref":    true,
	"branch": true,
	"tag":    true,
}

func ResolveCliParamsForFile(filePath string) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, errors.Wrap(err, "unable to read file")
	}

	resolvedContent, err := resolveCliParams(string(content))
	if err != nil {
		return false, err
	}

	if resolvedContent != string(content) {
		err = os.WriteFile(filePath, []byte(resolvedContent), 0644)
		if err != nil {
			return false, errors.Wrap(err, "unable to write file")
		}
		return true, nil
	}

	return false, nil
}

func resolveCliParams(yamlContent string) (string, error) {
	doc, err := ParseYAMLDoc(yamlContent)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse YAML")
	}

	if !doc.hasPath("$.on") {
		return "", errors.New("no git init params found in any trigger")
	}

	if !strings.Contains(yamlContent, "event.git.") {
		return "", errors.New("no git init params found in any trigger")
	}

	if doc.hasPath("$.on.cli.init") && strings.Contains(doc.TryReadStringAtPath("$.on.cli.init"), "event.git.") {
		return yamlContent, nil
	}

	gitParams, err := extractGitParams(doc)
	if err != nil {
		return "", err
	}
	if len(gitParams) == 0 {
		return yamlContent, nil
	}

	if doc.hasPath("$.on.cli.init") {
		err = doc.MergeAtPath("$.on.cli.init", gitParams)
		if err != nil {
			return "", err
		}
	} else {
		err = doc.MergeAtPath("$.on", map[string]any{
			"cli": map[string]any{
				"init": gitParams,
			},
		})
		if err != nil {
			return "", err
		}
	}

	result := doc.String()
	if strings.HasPrefix(yamlContent, "\n") && !strings.HasPrefix(result, "\n") {
		result = "\n" + result
	}

	return result, nil
}

func extractGitParams(doc *YAMLDoc) (map[string]any, error) {
	result := make(map[string]any)

	onNode, err := doc.getNodeAtPath("$.on")
	if err != nil {
		return result, nil
	}

	mappingNode, ok := onNode.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	for i := range mappingNode.Values {
		triggerEntry := mappingNode.Values[i]
		if triggerEntry.Key.String() == "cli" {
			continue
		}

		err := extractGitParamsFromTrigger(triggerEntry.Value, result)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func extractGitParamsFromTrigger(node ast.Node, result map[string]any) error {
	triggerNode, ok := node.(*ast.MappingNode)
	if !ok {
		return nil
	}

	for i := range triggerNode.Values {
		eventEntry := triggerNode.Values[i]
		err := extractGitParamsFromEvent(eventEntry.Value, result)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractGitParamsFromEvent(node ast.Node, result map[string]any) error {
	eventNode, ok := node.(*ast.MappingNode)
	if !ok {
		return nil
	}

	for i := range eventNode.Values {
		field := eventNode.Values[i]
		if field.Key.String() != "init" {
			continue
		}

		err := extractGitParamsFromInit(field.Value, result)
		if err != nil {
			return err
		}
	}
	return nil
}

func extractGitParamsFromInit(node ast.Node, result map[string]any) error {
	initNode, ok := node.(*ast.MappingNode)
	if !ok {
		return nil
	}

	for i := range initNode.Values {
		initParam := initNode.Values[i]
		paramName := initParam.Key.String()
		paramValue := initParam.Value.String()

		if gitInitParams[paramName] && strings.Contains(paramValue, "event.git.") {
			if existing, exists := result[paramName]; exists && existing != paramValue {
				return errors.Errorf("conflict: param %q has conflicting values", paramName)
			}
			result[paramName] = paramValue
		}
	}
	return nil
}
