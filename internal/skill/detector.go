package skill

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/rwx-cloud/rwx/internal/fs"
)

type Installation struct {
	Scope    string
	Path     string
	Version  string
	Detected bool
	Source   string
}

type DetectResult struct {
	Installations []Installation
	AnyFound      bool
}

// skillFrontmatter represents the YAML frontmatter in SKILL.md files.
// The version lives under metadata.version per the agent skills specification.
type skillFrontmatter struct {
	Metadata struct {
		Version string `yaml:"version"`
	} `yaml:"metadata"`
}

// Detect scans all known locations for installed RWX agent skills.
func Detect() (*DetectResult, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var installations []Installation

	// Check global agents installation
	globalPath := filepath.Join(homeDir, ".agents", "skills", "rwx", "SKILL.md")
	globalInst, err := checkSkillFile("global", "agents", globalPath)
	if err != nil {
		return nil, err
	}
	installations = append(installations, globalInst)

	// Check project agents installation (walk cwd up), skipping if it resolves
	// to the same file as the global installation.
	skillSubdir := filepath.Join(".agents", "skills", "rwx", "SKILL.md")
	if projectPath := findFileWalkingUp(cwd, skillSubdir); projectPath != "" {
		skip := false
		if globalInst.Detected {
			globalReal, err1 := filepath.EvalSymlinks(globalPath)
			projectReal, err2 := filepath.EvalSymlinks(projectPath)
			if err1 == nil && err2 == nil && globalReal == projectReal {
				skip = true
			}
		}
		if !skip {
			inst, err := checkSkillFile("project", "agents", projectPath)
			if err != nil {
				return nil, err
			}
			installations = append(installations, inst)
		}
	}

	// Check Claude Code marketplace installation
	marketplacePath := filepath.Join(homeDir, ".claude", "plugins", "marketplaces", "rwx", "plugins", "rwx", "skills", "rwx", "SKILL.md")
	marketplaceInst, err := checkSkillFile("global", "marketplace", marketplacePath)
	if err != nil {
		return nil, err
	}
	installations = append(installations, marketplaceInst)

	anyFound := false
	for _, inst := range installations {
		if IsDetected(inst) {
			anyFound = true
			break
		}
	}

	return &DetectResult{
		Installations: installations,
		AnyFound:      anyFound,
	}, nil
}

// checkSkillFile checks for a SKILL.md file and extracts the version from its frontmatter.
func checkSkillFile(scope, source, path string) (Installation, error) {
	inst := Installation{
		Scope:  scope,
		Path:   path,
		Source: source,
	}

	exists, err := fs.Exists(path)
	if err != nil {
		return inst, err
	}
	if !exists {
		return inst, nil
	}

	inst.Detected = true

	content, err := os.ReadFile(path)
	if err != nil {
		return inst, nil
	}

	inst.Version = parseSkillVersion(string(content))
	return inst, nil
}

// parseSkillVersion extracts the version from SKILL.md YAML frontmatter.
// Frontmatter is delimited by "---" lines.
func parseSkillVersion(content string) string {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return ""
	}

	frontmatter := parts[1]
	var fm skillFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
		return ""
	}

	return fm.Metadata.Version
}

// IsDetected returns true if the installation was actually found on disk.
func IsDetected(inst Installation) bool {
	return inst.Detected
}

// findFileWalkingUp walks from startDir up to the filesystem root looking for relPath.
// Returns the absolute path if found, or "" if not found.
func findFileWalkingUp(startDir, relPath string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, relPath)
		exists, err := fs.Exists(candidate)
		if err == nil && exists {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
