package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rwx-cloud/cli/internal/cli"
	"github.com/stretchr/testify/require"
)

func TestFindRunDefinitionFile(t *testing.T) {
	t.Run("when file exists in pwd", func(t *testing.T) {
		t.Chdir(t.TempDir())

		err := os.WriteFile("ci.yml", []byte{}, 0644)
		require.NoError(t, err)

		result, err := cli.FindRunDefinitionFile("ci.yml", "")
		require.NoError(t, err)
		require.Equal(t, "ci.yml", result)
	})

	t.Run("when file does not exist in pwd but exists in rwx directory", func(t *testing.T) {
		tmp := t.TempDir()
		t.Chdir(tmp)

		rwxDir := filepath.Join(tmp, ".rwx")
		err := os.Mkdir(rwxDir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(rwxDir, "ci.yml"), []byte{}, 0644)
		require.NoError(t, err)

		result, err := cli.FindRunDefinitionFile("ci.yml", rwxDir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(rwxDir, "ci.yml"), result)
	})

	t.Run("when file exists in both pwd and rwx directory, prefers pwd", func(t *testing.T) {
		tmp := t.TempDir()
		t.Chdir(tmp)

		rwxDir := filepath.Join(tmp, ".rwx")
		err := os.Mkdir(rwxDir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tmp, "ci.yml"), []byte{}, 0644)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(rwxDir, "ci.yml"), []byte{}, 0644)
		require.NoError(t, err)

		result, err := cli.FindRunDefinitionFile("ci.yml", rwxDir)
		require.NoError(t, err)
		require.Equal(t, "ci.yml", result)
	})

	t.Run("when file does not exist in either location", func(t *testing.T) {
		tmp := t.TempDir()
		t.Chdir(tmp)

		rwxDir := filepath.Join(tmp, ".rwx")
		err := os.Mkdir(rwxDir, 0755)
		require.NoError(t, err)

		_, err = cli.FindRunDefinitionFile("nonexistent.yml", rwxDir)
		require.Error(t, err)
		require.Equal(t, `run definition file "nonexistent.yml" not found in current directory or in "`+rwxDir+`"`, err.Error())
	})

	t.Run("when file does not exist and no rwx directory provided", func(t *testing.T) {
		t.Chdir(t.TempDir())

		_, err := cli.FindRunDefinitionFile("nonexistent.yml", "")
		require.Error(t, err)
		require.Equal(t, `run definition file "nonexistent.yml" not found`, err.Error())
	})

	t.Run("when absolute path is provided and exists", func(t *testing.T) {
		tmp := t.TempDir()

		filePath := filepath.Join(tmp, "ci.yml")
		err := os.WriteFile(filePath, []byte{}, 0644)
		require.NoError(t, err)

		result, err := cli.FindRunDefinitionFile(filePath, "")
		require.NoError(t, err)
		require.Equal(t, filePath, result)
	})

	t.Run("when absolute path is provided and does not exist", func(t *testing.T) {
		tmp := t.TempDir()

		filePath := filepath.Join(tmp, "nonexistent.yml")

		_, err := cli.FindRunDefinitionFile(filePath, "")
		require.Error(t, err)
		require.Equal(t, `run definition file "`+filePath+`" not found`, err.Error())
	})
}
