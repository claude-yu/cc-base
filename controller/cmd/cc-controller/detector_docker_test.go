package main

import "testing"

func TestIsUnderRoot(t *testing.T) {
	tests := []struct {
		name  string
		mount string
		root  string
		want  bool
	}{
		{"exact match", `D:\research-work\work-9`, `D:\research-work\work-9`, true},
		{"subdirectory", `D:\research-work\work-9\haddock`, `D:\research-work\work-9`, true},
		{"deep subdirectory", `D:\research-work\work-9\a\b\c`, `D:\research-work\work-9`, true},

		{"sibling directory", `D:\research-work\work-11`, `D:\research-work\work-9`, false},
		{"prefix but not subdir", `D:\research-work\work-99`, `D:\research-work\work-9`, false},
		{"completely different", `D:\other\path`, `D:\research-work\work-9`, false},

		{"case insensitive", `D:\research-work\Work-9\test`, `D:\research-work\work-9`, true},

		{"forward slashes", `D:/research-work/work-9/test`, `D:\research-work\work-9`, true},
		{"mixed slashes", `D:/research-work\work-9/test`, `D:\research-work\work-9`, true},

		{"empty mount", "", `D:\research-work\work-9`, false},
		{"empty root", `D:\research-work\work-9`, "", false},
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


