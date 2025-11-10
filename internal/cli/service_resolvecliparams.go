package cli

import (
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

func ResolveCliParams(yamlContent string) (string, error) {
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

	gitParams := extractGitParams(doc)
	if len(gitParams) == 0 {
		return yamlContent, nil
	}

	err = doc.MergeAtPath("$.on", map[string]any{
		"cli": map[string]any{
			"init": gitParams,
		},
	})
	if err != nil {
		return "", err
	}

	result := doc.String()
	if strings.HasPrefix(yamlContent, "\n") && !strings.HasPrefix(result, "\n") {
		result = "\n" + result
	}

	return result, nil
}

func extractGitParams(doc *YAMLDoc) map[string]any {
	result := make(map[string]any)

	onNode, err := doc.getNodeAtPath("$.on")
	if err != nil {
		return result
	}

	mappingNode, ok := onNode.(*ast.MappingNode)
	if !ok {
		return result
	}

	for i := range mappingNode.Values {
		triggerEntry := mappingNode.Values[i]
		if triggerEntry.Key.String() == "cli" {
			continue
		}

		extractGitParamsFromTrigger(triggerEntry.Value, result)
	}

	return result
}

func extractGitParamsFromTrigger(node ast.Node, result map[string]any) {
	triggerNode, ok := node.(*ast.MappingNode)
	if !ok {
		return
	}

	for i := range triggerNode.Values {
		eventEntry := triggerNode.Values[i]
		extractGitParamsFromEvent(eventEntry.Value, result)
	}
}

func extractGitParamsFromEvent(node ast.Node, result map[string]any) {
	eventNode, ok := node.(*ast.MappingNode)
	if !ok {
		return
	}

	for i := range eventNode.Values {
		field := eventNode.Values[i]
		if field.Key.String() != "init" {
			continue
		}

		extractGitParamsFromInit(field.Value, result)
	}
}

func extractGitParamsFromInit(node ast.Node, result map[string]any) {
	initNode, ok := node.(*ast.MappingNode)
	if !ok {
		return
	}

	for i := range initNode.Values {
		initParam := initNode.Values[i]
		paramName := initParam.Key.String()
		paramValue := initParam.Value.String()

		if gitInitParams[paramName] && strings.Contains(paramValue, "event.git.") {
			result[paramName] = paramValue
		}
	}
}
