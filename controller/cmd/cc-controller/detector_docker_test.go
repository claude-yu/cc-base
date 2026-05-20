package main

import "testing"

func TestIsUnderRoot(t *testing.T) {
	tests := []struct {
		name  string
		mount string
		root  string
		want  bool
	}{
		{"exact match", `G:\proteinwork\work-9`, `G:\proteinwork\work-9`, true},
		{"subdirectory", `G:\proteinwork\work-9\haddock`, `G:\proteinwork\work-9`, true},
		{"deep subdirectory", `G:\proteinwork\work-9\a\b\c`, `G:\proteinwork\work-9`, true},

		{"sibling directory", `G:\proteinwork\work-11`, `G:\proteinwork\work-9`, false},
		{"prefix but not subdir", `G:\proteinwork\work-99`, `G:\proteinwork\work-9`, false},
		{"completely different", `D:\other\path`, `G:\proteinwork\work-9`, false},

		{"case insensitive", `g:\Proteinwork\Work-9\test`, `G:\proteinwork\work-9`, true},

		{"forward slashes", `G:/proteinwork/work-9/test`, `G:\proteinwork\work-9`, true},
		{"mixed slashes", `G:/proteinwork\work-9/test`, `G:\proteinwork\work-9`, true},

		{"empty mount", "", `G:\proteinwork\work-9`, false},
		{"empty root", `G:\proteinwork\work-9`, "", false},
		{"both empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderRoot(tt.mount, tt.root)
			if got != tt.want {
				t.Errorf("isUnderRoot(%q, %q) = %v, want %v",
					tt.mount, tt.root, got, tt.want)
			}
		})
	}
}
