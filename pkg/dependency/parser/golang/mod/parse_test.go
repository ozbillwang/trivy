package mod

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/modfile"

	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		replace       bool
		useMinVersion bool
		want          []ftypes.Package
	}{
		{
			name:          "normal with stdlib",
			file:          "testdata/normal/go.mod",
			replace:       true,
			useMinVersion: true,
			want:          GoModNormal,
		},
		{
			name:    "normal",
			file:    "testdata/normal/go.mod",
			replace: true,
			want:    GoModNormalWithoutStdlib,
		},
		{
			name:    "without go version",
			file:    "testdata/no-go-version/gomod",
			replace: true,
			want:    GoModNoGoVersion,
		},
		{
			name:    "replace",
			file:    "testdata/replaced/go.mod",
			replace: true,
			want:    GoModReplaced,
		},
		{
			name:    "no replace",
			file:    "testdata/replaced/go.mod",
			replace: false,
			want:    GoModUnreplaced,
		},
		{
			name:    "replace with version",
			file:    "testdata/replaced-with-version/go.mod",
			replace: true,
			want:    GoModReplacedWithVersion,
		},
		{
			name:    "replaced with version mismatch",
			file:    "testdata/replaced-with-version-mismatch/go.mod",
			replace: true,
			want:    GoModReplacedWithVersionMismatch,
		},
		{
			name:    "replaced with local path",
			file:    "testdata/replaced-with-local-path/go.mod",
			replace: true,
			want:    GoModReplacedWithLocalPath,
		},
		{
			name:    "replaced with local path and version",
			file:    "testdata/replaced-with-local-path-and-version/go.mod",
			replace: true,
			want:    GoModReplacedWithLocalPathAndVersion,
		},
		{
			name:    "replaced with local path and version, mismatch",
			file:    "testdata/replaced-with-local-path-and-version-mismatch/go.mod",
			replace: true,
			want:    GoModReplacedWithLocalPathAndVersionMismatch,
		},
		{
			name:    "go 1.16",
			file:    "testdata/go116/go.mod",
			replace: true,
			want:    GoMod116,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.file)
			require.NoError(t, err)

			got, _, err := NewParser(tt.replace, tt.useMinVersion).Parse(f)
			require.NoError(t, err)

			sort.Sort(ftypes.Packages(got))
			sort.Sort(ftypes.Packages(tt.want))

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToolchainVersion(t *testing.T) {
	tests := []struct {
		name    string
		modFile modfile.File
		want    string
	}{
		{
			name: "version from toolchain line",
			modFile: modfile.File{
				Toolchain: &modfile.Toolchain{
					Name: "1.21.1",
				},
			},
			want: "1.21.1",
		},
		{
			name: "version from toolchain line with suffix",
			modFile: modfile.File{
				Toolchain: &modfile.Toolchain{
					Name: "1.21.1-custom",
				},
			},
			want: "1.21.1",
		},
		{
			name: "'1.18rc1' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.18rc1",
				},
			},
			want: "",
		},
		{
			name: "'1.18.1' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.18.1",
				},
			},
			want: "",
		},
		{
			name: "'1.20' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.20",
				},
			},
			want: "",
		},
		{
			name: "'1.21' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.21",
				},
			},
			want: "1.21.0",
		},
		{
			name: "'1.21rc1' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.21rc1",
				},
			},
			want: "1.21rc1",
		},
		{
			name: "'1.21.2' from go line",
			modFile: modfile.File{
				Go: &modfile.Go{
					Version: "1.21.2",
				},
			},
			want: "1.21.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, toolchainVersion(tt.modFile.Toolchain, tt.modFile.Go))
		})
	}
}
