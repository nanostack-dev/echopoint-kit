package apispec_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanostack-dev/echopoint-kit/apispec"
)

var update = flag.Bool("update", false, "rewrite the golden files of the current layout version")

const canonicalSuffix = ".canonical.yaml"

func layoutDir() string {
	return filepath.Join("testdata", "canonical", fmt.Sprintf("v%d", apispec.LayoutVersion))
}

// TestCanonicalLayoutGolden writes each fixture in the canonical layout and
// compares it with its golden file. The goldens live in a directory named
// after LayoutVersion: a change to the canonical output fails here until it
// is shipped as a new layout version with its own goldens.
func TestCanonicalLayoutGolden(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join(layoutDir(), "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fixtures := 0
	for _, input := range inputs {
		if strings.HasSuffix(input, canonicalSuffix) {
			continue
		}
		fixtures++
		name := strings.TrimSuffix(filepath.Base(input), ".yaml")
		t.Run(name, func(t *testing.T) {
			got := canonicalOf(t, readFile(t, input))
			golden := strings.TrimSuffix(input, ".yaml") + canonicalSuffix
			if *update {
				writeFile(t, golden, got)
			}
			if want := readFile(t, golden); !bytes.Equal(got, want) {
				t.Errorf("canonical layout of %s differs from %s:\n%s", input, golden, got)
			}
		})
	}
	if fixtures == 0 {
		t.Fatalf("no fixtures in %s: LayoutVersion changed without its goldens", layoutDir())
	}
}

func TestCanonicalLayoutIsStableWhenReapplied(t *testing.T) {
	goldens, err := filepath.Glob(filepath.Join(layoutDir(), "*"+canonicalSuffix))
	if err != nil {
		t.Fatal(err)
	}
	for _, golden := range goldens {
		t.Run(filepath.Base(golden), func(t *testing.T) {
			want := readFile(t, golden)
			if got := canonicalOf(t, want); !bytes.Equal(got, want) {
				t.Errorf("reapplying the canonical layout changed %s:\n%s", golden, got)
			}
		})
	}
}

func TestCanonicalLayoutOfStripeSizedDocument(t *testing.T) {
	if testing.Short() {
		t.Skip("parses a 6.6 MB document")
	}
	canonical := canonicalOf(t, gunzip(t, filepath.Join("testdata", "stripe", "spec3.yaml.gz")))

	sum := sha256.Sum256(canonical)
	got := hex.EncodeToString(sum[:])
	goldenPath := filepath.Join("testdata", "stripe", fmt.Sprintf("spec3.canonical.v%d.sha256", apispec.LayoutVersion))
	if *update {
		writeFile(t, goldenPath, []byte(got+"\n"))
	}
	if want := strings.TrimSpace(string(readFile(t, goldenPath))); got != want {
		t.Errorf("canonical layout of the Stripe document changed: sha256 %s, golden %s", got, want)
	}
	if again := canonicalOf(t, canonical); !bytes.Equal(again, canonical) {
		t.Error("reapplying the canonical layout changed the Stripe document")
	}
}

func canonicalOf(t *testing.T, data []byte) []byte {
	t.Helper()
	document, err := apispec.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canonical, err := document.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	return canonical
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func gunzip(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
