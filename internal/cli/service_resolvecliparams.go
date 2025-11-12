package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/rwx-cloud/cli/internal/errors"
)

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

	gitParams, err := extractGitParams(doc)
	if err != nil {
		return "", err
	}
	if len(gitParams) == 0 {
		return yamlContent, nil
	}

	// Skip if CLI init already has git event references
	if cliInit := doc.TryReadStringAtPath("$.on.cli.init"); strings.Contains(cliInit, "event.git.") {
		return yamlContent, nil
	}

	// Create new 'on' section if it doesn't exist
	if !doc.hasPath("$.on") {
		return prependOnSection(yamlContent, gitParams), nil
	}

	if doc.hasPath("$.on.cli.init") {
		err = doc.MergeAtPath("$.on.cli.init", gitParams)
	} else {
		err = doc.MergeAtPath("$.on", map[string]any{
			"cli": map[string]any{
				"init": gitParams,
			},
		})
	}
	if err != nil {
		return "", err
	}

	result := doc.String()
	if strings.HasPrefix(yamlContent, "\n") && !strings.HasPrefix(result, "\n") {
		result = "\n" + result
	}

	return result, nil
}

func prependOnSection(yamlContent string, params map[string]any) string {
	var onSection strings.Builder
	onSection.WriteString("on:\n  cli:\n    init:\n")

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		onSection.WriteString(fmt.Sprintf("      %s: %s\n", k, params[k]))
	}

	return onSection.String() + yamlContent
}

func extractGitParams(doc *YAMLDoc) (map[string]any, error) {
	result := make(map[string]any)

	onNode, err := doc.getNodeAtPath("$.on")
	if err == nil {
		mappingNode, ok := onNode.(*ast.MappingNode)
		if ok {
			for i := range mappingNode.Values {
				triggerEntry := mappingNode.Values[i]
				if triggerEntry.Key.String() == "cli" {
					continue
				}

				result, err = extractGitParamsFromTrigger(triggerEntry.Value, result)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	result, err = extractGitParamsFromGitClone(doc, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func extractGitParamsFromTrigger(node ast.Node, result map[string]any) (map[string]any, error) {
	if sequenceNode, ok := node.(*ast.SequenceNode); ok {
		for _, element := range sequenceNode.Values {
			var err error
			result, err = extractGitParamsFromEvent(element, result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	triggerNode, ok := node.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	for i := range triggerNode.Values {
		var err error
		result, err = extractGitParamsFromEvent(triggerNode.Values[i].Value, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func extractGitParamsFromEvent(node ast.Node, result map[string]any) (map[string]any, error) {
	if sequenceNode, ok := node.(*ast.SequenceNode); ok {
		for _, element := range sequenceNode.Values {
			var err error
			result, err = extractGitParamsFromEvent(element, result)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	eventNode, ok := node.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	for i := range eventNode.Values {
		field := eventNode.Values[i]
		if field.Key.String() == "init" {
			return extractGitParamsFromInit(field.Value, result)
		}
	}
	return result, nil
}

func extractGitParamsFromInit(node ast.Node, result map[string]any) (map[string]any, error) {
	initNode, ok := node.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	for i := range initNode.Values {
		initParam := initNode.Values[i]
		paramName := initParam.Key.String()
		paramValue := initParam.Value.String()

		if strings.Contains(paramValue, "event.git.ref") || strings.Contains(paramValue, "event.git.sha") {
			// Always map to event.git.sha for CLI trigger
			result[paramName] = "${{ event.git.sha }}"
		}
	}
	return result, nil
}

func extractGitParamsFromGitClone(doc *YAMLDoc, result map[string]any) (map[string]any, error) {
	tasksNode, err := doc.getNodeAtPath("$.tasks")
	if err != nil {
		return result, nil
	}

	sequenceNode, ok := tasksNode.(*ast.SequenceNode)
	if !ok {
		return result, nil
	}

	var gitCloneRefParam string

	for i := range sequenceNode.Values {
		callValue := doc.TryReadStringAtPath(fmt.Sprintf("$.tasks[%d].call", i))
		if !strings.HasPrefix(callValue, "git/clone") {
			continue
		}

		refValue := doc.TryReadStringAtPath(fmt.Sprintf("$.tasks[%d].with.ref", i))
		if refValue == "" || !strings.Contains(refValue, "init.") {
			continue
		}

		parts := strings.Split(refValue, "init.")
		if len(parts) < 2 {
			continue
		}

		paramName := strings.TrimSpace(parts[1])
		paramName = strings.TrimRight(paramName, " })")

		if paramName == "" {
			continue
		}

		if gitCloneRefParam != "" && gitCloneRefParam != paramName {
			return nil, errors.New("multiple git/clone packages use different ref init params")
		}
		gitCloneRefParam = paramName
	}

	if gitCloneRefParam == "" {
		return result, nil
	}

	// Always map to event.git.sha for CLI trigger
	result[gitCloneRefParam] = "${{ event.git.sha }}"

	return result, nil
}
