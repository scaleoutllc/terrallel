package terrallel_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/scaleoutllc/terrallel/internal/terrallel"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		manifest    string
		imports     map[string]string
		expected    map[string]*terrallel.Target
		expectedErr string
	}{
		{
			name:        "no manifest",
			expectedErr: "reading",
		},
		{
			name: "no imported files",
			manifest: `
targets:
  t1:
    workspaces:
    - t1ws1
    - t1ws2
    next:
      workspaces:
      - t1ws3`,
			imports: map[string]string{},
			expected: map[string]*terrallel.Target{
				"t1": {
					Name:       "t1",
					Workspaces: []string{"t1ws1", "t1ws2"},
					Next: &terrallel.Target{
						Name:       "next",
						Workspaces: []string{"t1ws3"},
					},
				},
			},
		},
		{
			name: "invalid manifest (group and workspaces at the same level)",
			manifest: `
targets:
  t1:
    workspaces:
    - t1ws1
    group:
    - t2
`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "coexist at the same level",
		},
		{
			name: "malformed yaml",
			manifest: `
	targets:
`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "parsing manifest",
		},
		{
			name: "malformed yaml in imports",
			manifest: `
terrallel:
  import:
  - bad.yml
`,
			imports: map[string]string{
				"bad.yml": `
 123
     asd
`,
			},
			expected:    nil,
			expectedErr: "parsing imports",
		},
		{
			name: "group references non-existent target",
			manifest: `
targets:
  t1:
    workspaces:
    - t1ws1
    next:
      group:
      - t2
`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "resolving targets:",
		},
		{
			name: "missing import file",
			manifest: `
terrallel:
  import:
  - missing.yml`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "reading import files:",
		},
		{
			name: "bad import glob",
			manifest: `
terrallel:
  import:
  - "["`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "expanding file path:",
		},
		{
			name: "duplicate target in imports",
			manifest: `
terrallel:
  import:
  - "*.yml"`,
			imports: map[string]string{
				"1.yml": `
targets:
  t2:
    workspaces:
    - t2ws1
    - t2ws2`,
				"2.yml": `
targets:
  t2:
    workspaces:
    - t2ws1`,
			},
			expected:    nil,
			expectedErr: "duplicate",
		},
		{
			name: "recursive target loop",
			manifest: `
targets:
  t1:
    group:
    - t2
  t2:
    group:
    - t1`,
			imports:     map[string]string{},
			expected:    nil,
			expectedErr: "recursive loop",
		},
		{
			name: "valid with imports",
			manifest: `
terrallel:
  import:
  - "*.yml"
targets:
  t1:
    workspaces:
    - t1ws1
    - t1ws2
    next:
      workspaces:
      - t1ws3`,
			imports: map[string]string{
				"1.yml": `
targets:
  t2:
    workspaces:
    - t2ws1
    - t2ws2
    next:
      group:
      - t3`,
				"2.yml": `
targets:
  t3:
    workspaces:
    - t3ws1
    - t3ws2`,
			},
			expected: map[string]*terrallel.Target{
				"t1": {
					Name:       "t1",
					Workspaces: []string{"t1ws1", "t1ws2"},
					Next: &terrallel.Target{
						Name:       "next",
						Workspaces: []string{"t1ws3"},
					},
				},
				"t2": {
					Name:       "t2",
					Workspaces: []string{"t2ws1", "t2ws2"},
					Next: &terrallel.Target{
						Name: "next",
						Group: []*terrallel.Target{
							{
								Name:       "t3",
								Workspaces: []string{"t3ws1", "t3ws2"},
							},
						},
					},
				},
				"t3": {
					Name:       "t3",
					Workspaces: []string{"t3ws1", "t3ws2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			defer os.RemoveAll(tempDir)
			manifestPath := filepath.Join(tempDir, "manifest")
			if tt.manifest != "" {
				if err := os.WriteFile(manifestPath, []byte(tt.manifest), 0644); err != nil {
					t.Fatalf("failed to write main manifest: %v", err)
				}
			}
			for filename, content := range tt.imports {
				err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to write import file %s: %v", filename, err)
				}
			}
			tl := terrallel.New(io.Discard, io.Discard)
			err := tl.Load(manifestPath)
			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %s but got error: %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unxpected error: %v", err)
				}
				if diff := cmp.Diff(tt.expected, tl.Manifest); diff != "" {
					t.Errorf("manifest mismatch (-expected +actual):\n%s", diff)
				}
			}
		})
	}
}
