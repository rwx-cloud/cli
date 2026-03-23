package versions

import (
	"os"
	"strings"
	"sync"

	semver "github.com/Masterminds/semver/v3"
	"github.com/rwx-cloud/rwx/cmd/rwx/config"
)

var versionHolder *lockedVersions

var EmptyVersion = semver.MustParse("0.0.0")

const latestVersionFilename = "latestversion"
const latestSkillVersionFilename = "latestskillversion"

type lockedVersions struct {
	currentVersion     *semver.Version
	latestVersion      *semver.Version
	latestSkillVersion *semver.Version
	mu                 sync.RWMutex
}

func init() {
	currentVersion, err := semver.NewVersion(config.Version)
	if err != nil {
		// Assume this is a development build and it is newer than any release.
		currentVersion = semver.MustParse("9999+" + config.Version)
	}

	versionHolder = &lockedVersions{
		currentVersion:     currentVersion,
		latestVersion:      EmptyVersion,
		latestSkillVersion: EmptyVersion,
	}
}

func GetCliCurrentVersion() *semver.Version {
	return versionHolder.currentVersion
}

func GetCliLatestVersion() *semver.Version {
	versionHolder.mu.RLock()
	defer versionHolder.mu.RUnlock()

	return versionHolder.latestVersion
}

func SetCliLatestVersion(versionStr string) error {
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return err
	}

	versionHolder.mu.Lock()
	versionHolder.latestVersion = version
	versionHolder.mu.Unlock()

	return nil
}

func NewVersionAvailable() bool {
	currentVersion := GetCliCurrentVersion()
	latestVersion := GetCliLatestVersion()

	return latestVersion.GreaterThan(currentVersion)
}

func GetSkillLatestVersion() *semver.Version {
	versionHolder.mu.RLock()
	defer versionHolder.mu.RUnlock()

	return versionHolder.latestSkillVersion
}

func SetSkillLatestVersion(versionStr string) error {
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return err
	}

	versionHolder.mu.Lock()
	versionHolder.latestSkillVersion = version
	versionHolder.mu.Unlock()

	return nil
}

func InstalledWithHomebrew() bool {
	fname, err := os.Executable()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(fname), "/homebrew/")
}

func LoadLatestVersionFromFile(backend Backend) {
	if backend == nil {
		return
	}

	versionStr, err := backend.Get()
	if err != nil || versionStr == "" {
		return
	}

	_ = SetCliLatestVersion(versionStr)
}

func SaveLatestVersionToFile(backend Backend) {
	if backend == nil {
		return
	}

	latestVersion := GetCliLatestVersion()
	if latestVersion.Equal(EmptyVersion) {
		return
	}

	_ = backend.Set(latestVersion.String())
}

func LoadLatestSkillVersionFromFile(backend Backend) {
	if backend == nil {
		return
	}

	versionStr, err := backend.Get()
	if err != nil || versionStr == "" {
		return
	}

	_ = SetSkillLatestVersion(versionStr)
}

func SaveLatestSkillVersionToFile(backend Backend) {
	if backend == nil {
		return
	}

	latestVersion := GetSkillLatestVersion()
	if latestVersion.Equal(EmptyVersion) {
		return
	}

	_ = backend.Set(latestVersion.String())
}
