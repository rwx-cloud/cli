package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/rwx-cloud/cli/internal/errors"
)

var gitInitParams = map[string]bool{
	"sha": true,
	"ref": true,
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

	hasOnSection := doc.hasPath("$.on")

	if !hasOnSection {
		var onSection strings.Builder
		onSection.WriteString("on:\n  cli:\n    init:\n")

		keys := make([]string, 0, len(gitParams))
		for k := range gitParams {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		for _, k := range keys {
			onSection.WriteString(fmt.Sprintf("      %s: %s\n", k, gitParams[k]))
		}

		return onSection.String() + yamlContent, nil
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
		eventEntry := triggerNode.Values[i]
		var err error
		result, err = extractGitParamsFromEvent(eventEntry.Value, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func extractGitParamsFromEvent(node ast.Node, result map[string]any) (map[string]any, error) {
	eventNode, ok := node.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	for i := range eventNode.Values {
		field := eventNode.Values[i]
		if field.Key.String() != "init" {
			continue
		}

		var err error
		result, err = extractGitParamsFromInit(field.Value, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func extractGitParamsFromInit(node ast.Node, result map[string]any) (map[string]any, error) {
	initNode, ok := node.(*ast.MappingNode)
	if !ok {
		return result, nil
	}

	newResult := make(map[string]any)
	for k, v := range result {
		newResult[k] = v
	}

	for i := range initNode.Values {
		initParam := initNode.Values[i]
		paramName := initParam.Key.String()
		paramValue := initParam.Value.String()

		if strings.Contains(paramValue, "event.git.ref") || strings.Contains(paramValue, "event.git.sha") {
			// Always map to event.git.sha for CLI trigger
			newResult[paramName] = "${{ event.git.sha }}"
		}
	}
	return newResult, nil
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

	for _, taskNode := range sequenceNode.Values {
		mappingNode, ok := taskNode.(*ast.MappingNode)
		if !ok {
			continue
		}

		var isGitClone bool
		var refValue string

		for i := range mappingNode.Values {
			entry := mappingNode.Values[i]
			key := entry.Key.String()

			if key == "call" && strings.HasPrefix(entry.Value.String(), "git/clone") {
				isGitClone = true
			}

			if key == "with" {
				withNode, ok := entry.Value.(*ast.MappingNode)
				if !ok {
					continue
				}

				for j := range withNode.Values {
					withEntry := withNode.Values[j]
					if withEntry.Key.String() == "ref" {
						refValue = withEntry.Value.String()
					}
				}
			}
		}

		if isGitClone && refValue != "" {
			if strings.Contains(refValue, "init.") {
				parts := strings.Split(refValue, "init.")
				if len(parts) >= 2 {
					paramName := strings.TrimSpace(parts[1])
					paramName = strings.TrimRight(paramName, " })")

					if gitInitParams[paramName] {
						if gitCloneRefParam != "" && gitCloneRefParam != paramName {
							return nil, errors.New("multiple git/clone packages use different ref init params")
						}
						gitCloneRefParam = paramName
					}
				}
			}
		}
	}

	if gitCloneRefParam == "" {
		return result, nil
	}

	newResult := make(map[string]any)
	for k, v := range result {
		newResult[k] = v
	}

	// Always map to event.git.sha for CLI trigger
	targetValue := "${{ event.git.sha }}"
	newResult[gitCloneRefParam] = targetValue

	return newResult, nil
}
