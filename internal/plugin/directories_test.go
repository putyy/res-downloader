package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundledPluginDirectoriesAreValid(t *testing.T) {
	testPluginDirectories(t, "bundled", true)
}

func TestExamplePluginDirectoriesAreValid(t *testing.T) {
	testPluginDirectories(t, filepath.Join("..", "..", "examples", "plugins"), false)
}

func testPluginDirectories(t *testing.T, root string, bundled bool) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			directory := filepath.Join(root, entry.Name())
			loader := LoadExternalPlugin
			if bundled {
				loader = LoadBundledPlugin
			}
			if _, _, err := loader(directory); err != nil {
				t.Fatal(err)
			}
			testPluginFixtures(t, directory)
		})
	}
}

func testPluginFixtures(t *testing.T, directory string) {
	t.Helper()
	fixtureDirectory := filepath.Join(directory, "fixtures")
	entries, err := os.ReadDir(fixtureDirectory)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		t.Run("fixture/"+entry.Name(), func(t *testing.T) {
			if err := ReplayPluginFixture(directory, filepath.Join(fixtureDirectory, entry.Name())); err != nil {
				t.Fatal(err)
			}
		})
	}
}
