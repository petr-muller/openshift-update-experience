package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func compareWithFixture(t *testing.T, actualOut []byte, fixtureDir string, outputSuffix string) {
	t.Helper()
	expectedOutPath := filepath.Join(fixtureDir, "expected"+outputSuffix)

	if update := os.Getenv("UPDATE"); update != "" {
		if err := os.WriteFile(expectedOutPath, actualOut, 0644); err != nil {
			t.Fatalf("Error when writing output fixture: %v", err)
		}
		return
	}

	expectedOut, err := os.ReadFile(expectedOutPath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("Error when reading output fixture: %v", err)
		} else {
			t.Fatalf("Output file %s does not exist. You may rerun this test with UPDATE=true to create output file with the following actual output:\n%s", expectedOutPath, actualOut)
		}
	}

	if diff := cmp.Diff(string(expectedOut), string(actualOut)); diff != "" {
		t.Errorf("Output differs from expected (%s):\n%s", filepath.Base(expectedOutPath), diff)
	}
}

func TestExamples(t *testing.T) {
	examples, err := filepath.Glob("examples/*")
	if err != nil {
		t.Fatalf("Error when listing examples: %v", err)
	}

	variants := []struct {
		name         string
		detailed     string
		outputSuffix string
	}{
		{
			name:         "normal output",
			detailed:     "none",
			outputSuffix: ".output",
		},
		{
			name:         "detailed output",
			detailed:     "all",
			outputSuffix: ".detailed-output",
		},
	}

	for _, example := range examples {
		for _, variant := range variants {
			t.Run(fmt.Sprintf("%s-%s", example, variant.name), func(t *testing.T) {
				t.Parallel()
				opts := &options{
					mockData:       mockData{path: example},
					detailedOutput: variant.detailed,
				}
				if err := opts.Complete(nil, nil); err != nil {
					t.Fatalf("Error when completing options: %v", err)
				}

				var stdout, stderr bytes.Buffer
				opts.Out = &stdout
				opts.ErrOut = &stderr

				if err := opts.Run(context.Background()); err != nil {
					t.Fatalf("Error when running: %v", err)
				}

				compareWithFixture(t, stdout.Bytes(), example, variant.outputSuffix)
			})
		}
	}
}
