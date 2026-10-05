package apispec_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

const editDir = "testdata/edit"

func editCases(t *testing.T) []string {
	t.Helper()
	cases, err := filepath.Glob(filepath.Join(editDir, "*", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatalf("no edit cases in %s", editDir)
	}
	return cases
}

func loadCommands(t *testing.T, path string) []apispec.Command {
	t.Helper()
	var commands []apispec.Command
	if err := json.Unmarshal(readFile(t, path), &commands); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return commands
}

func TestEditGolden(t *testing.T) {
	for _, commandsPath := range editCases(t) {
		dir := filepath.Dir(commandsPath)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			input := inputPath(t, dir)
			got, err := apispec.Edit(readFile(t, input), loadCommands(t, commandsPath)...)
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join(dir, "output"+filepath.Ext(input))
			if *update {
				writeFile(t, golden, got)
			}
			if want := readFile(t, golden); !bytes.Equal(got, want) {
				t.Errorf("edit of %s differs from %s:\n%s", input, golden, lineDiff(string(want), string(got)))
			}
			diffPath := filepath.Join(dir, "changes.diff")
			diff := lineDiff(string(readFile(t, input)), string(got))
			if *update {
				writeFile(t, diffPath, []byte(diff))
			}
			if want := readFile(t, diffPath); diff != string(want) {
				t.Errorf("the hunks of %s changed:\n%s", dir, diff)
			}
		})
	}
}

func inputPath(t *testing.T, dir string) string {
	t.Helper()
	for _, name := range []string{"input.yaml", "input.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return filepath.Join(dir, name)
		}
	}
	t.Fatalf("no input in %s", dir)
	return ""
}

func TestEditCanonicalParity(t *testing.T) {
	for _, commandsPath := range editCases(t) {
		dir := filepath.Dir(commandsPath)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			input := readFile(t, inputPath(t, dir))
			commands := loadCommands(t, commandsPath)
			edited, err := apispec.Edit(input, commands...)
			if err != nil {
				t.Fatal(err)
			}
			editedCanonical, err := apispec.Edit(canonicalOf(t, input), commands...)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := canonicalOf(t, edited), canonicalOf(t, editedCanonical); !bytes.Equal(got, want) {
				t.Errorf("Canonical(Edit(original)) differs from Canonical(Edit(Canonical(original))):\n%s",
					lineDiff(string(want), string(got)))
			}
		})
	}
}
