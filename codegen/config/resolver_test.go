package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolverConfig(t *testing.T) {
	t.Run("single-file", func(t *testing.T) {
		t.Run("when given just a filename", func(t *testing.T) {
			p := ResolverConfig{Filename: "testdata/example.go"}
			require.True(t, p.IsDefined())

			require.NoError(t, p.Check())
			require.NoError(t, p.Check())

			require.Equal(t, "config_test_data", p.Package)
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.ImportPath())

			require.Equal(t, "config_test_data", p.Pkg().Name())
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.Pkg().Path())

			require.Contains(t, filepath.ToSlash(p.Filename), "codegen/config/testdata/example.go")
			require.Contains(t, filepath.ToSlash(p.Dir()), "codegen/config/testdata")
		})

		t.Run("when given both", func(t *testing.T) {
			p := ResolverConfig{Filename: "testdata/example.go", Package: "wololo"}
			require.True(t, p.IsDefined())

			require.NoError(t, p.Check())
			require.NoError(t, p.Check())

			require.Equal(t, "wololo", p.Package)
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.ImportPath())

			require.Equal(t, "wololo", p.Pkg().Name())
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.Pkg().Path())

			require.Contains(t, filepath.ToSlash(p.Filename), "codegen/config/testdata/example.go")
			require.Contains(t, filepath.ToSlash(p.Dir()), "codegen/config/testdata")
		})

		t.Run("when given nothing", func(t *testing.T) {
			p := ResolverConfig{}
			require.False(t, p.IsDefined())

			require.EqualError(t, p.Check(), "filename must be specified with layout=single-file")

			require.Empty(t, p.Package)
			require.Empty(t, p.ImportPath())

			require.Nil(t, p.Pkg())

			require.Empty(t, p.Filename)
			require.Empty(t, p.Dir())
		})

		t.Run("when given invalid filename", func(t *testing.T) {
			p := ResolverConfig{Filename: "wololo.sql"}
			require.True(t, p.IsDefined())

			require.EqualError(
				t,
				p.Check(),
				"filename should be path to a go source file with layout=single-file",
			)
		})

		t.Run("when package includes a filename", func(t *testing.T) {
			p := ResolverConfig{Filename: "foo.go", Package: "foo/foo.go"}
			require.True(t, p.IsDefined())

			require.EqualError(
				t,
				p.Check(),
				"package should be the output package name only, do not include the output filename",
			)
		})
	})

	t.Run("follow-schema", func(t *testing.T) {
		t.Run("when given just a dir", func(t *testing.T) {
			p := ResolverConfig{Layout: LayoutFollowSchema, DirName: "testdata"}
			require.True(t, p.IsDefined())

			require.NoError(t, p.Check())
			require.NoError(t, p.Check())

			require.Equal(t, "config_test_data", p.Package)
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.ImportPath())

			require.Equal(t, "config_test_data", p.Pkg().Name())
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.Pkg().Path())

			require.Contains(t, filepath.ToSlash(p.Filename), "codegen/config/testdata/resolver.go")
			require.Contains(t, p.Dir(), "codegen/config/testdata")
		})

		t.Run("when given dir and package name", func(t *testing.T) {
			p := ResolverConfig{Layout: LayoutFollowSchema, DirName: "testdata", Package: "wololo"}
			require.True(t, p.IsDefined())

			require.NoError(t, p.Check())
			require.NoError(t, p.Check())

			require.Equal(t, "wololo", p.Package)
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.ImportPath())

			require.Equal(t, "wololo", p.Pkg().Name())
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.Pkg().Path())

			require.Contains(t, filepath.ToSlash(p.Filename), "codegen/config/testdata/resolver.go")
			require.Contains(t, p.Dir(), "codegen/config/testdata")
		})

		t.Run("when given a filename", func(t *testing.T) {
			p := ResolverConfig{
				Layout:   LayoutFollowSchema,
				DirName:  "testdata",
				Filename: "testdata/asdf.go",
			}
			require.True(t, p.IsDefined())

			require.NoError(t, p.Check())
			require.NoError(t, p.Check())

			require.Equal(t, "config_test_data", p.Package)
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.ImportPath())

			require.Equal(t, "config_test_data", p.Pkg().Name())
			require.Equal(t, "github.com/99designs/gqlgen/codegen/config/testdata", p.Pkg().Path())

			require.Contains(t, filepath.ToSlash(p.Filename), "codegen/config/testdata/asdf.go")
			require.Contains(t, p.Dir(), "codegen/config/testdata")
		})

		t.Run("when given nothing", func(t *testing.T) {
			p := ResolverConfig{Layout: LayoutFollowSchema}
			require.False(t, p.IsDefined())

			require.EqualError(t, p.Check(), "dirname must be specified with layout=follow-schema")

			require.Empty(t, p.Package)
			require.Empty(t, p.ImportPath())

			require.Nil(t, p.Pkg())

			require.Empty(t, p.Filename)
			require.Empty(t, p.Dir())
		})
	})

	t.Run("invalid layout", func(t *testing.T) {
		p := ResolverConfig{Layout: "pies", Filename: "asdf.go"}
		require.True(t, p.IsDefined())

		require.EqualError(
			t,
			p.Check(),
			"invalid layout pies. must be single-file, follow-schema or follow-object",
		)
	})
}

func TestResolverConfigFollowObject(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		p := ResolverConfig{Layout: LayoutFollowObject, DirName: "testdata"}
		require.NoError(t, p.Check())

		require.Equal(t, "{field}_{object}.go", p.OperationsFilenameTemplate)
		require.Equal(t, "{object}_fields.go", p.FieldsFilenameTemplate)
		require.Equal(t, "resolver_generated.go", p.TypesFilename)
		require.Contains(t, filepath.ToSlash(p.Filename), "testdata/resolver.go")
		require.Equal(t, p.DirName, p.Dir())
	})

	t.Run("fields can have a file each", func(t *testing.T) {
		p := ResolverConfig{
			Layout:                 LayoutFollowObject,
			DirName:                "testdata",
			FieldsFilenameTemplate: "{object}_{field}_resolver.go",
		}
		require.NoError(t, p.Check())
	})

	t.Run("needs a dir", func(t *testing.T) {
		p := ResolverConfig{Layout: LayoutFollowObject}
		require.ErrorContains(t, p.Check(), "dirname must be specified")
	})

	t.Run("filename_template belongs to follow-schema", func(t *testing.T) {
		p := ResolverConfig{
			Layout:           LayoutFollowObject,
			DirName:          "testdata",
			FilenameTemplate: "{name}.go",
		}
		require.ErrorContains(t, p.Check(), "filename_template is not used")
	})

	for name, tt := range map[string]struct {
		cfg     ResolverConfig
		wantErr string
	}{
		"unknown placeholder": {
			ResolverConfig{OperationsFilenameTemplate: "{name}_{field}.go"},
			"unknown placeholder {name}",
		},
		"operations must name the field": {
			ResolverConfig{OperationsFilenameTemplate: "{object}.go"},
			"must contain {field}",
		},
		"fields must name the object": {
			ResolverConfig{FieldsFilenameTemplate: "fields.go"},
			"must contain {object}",
		},
		"fields have no name placeholder": {
			ResolverConfig{FieldsFilenameTemplate: "{name}_{object}.go"},
			"unknown placeholder {name}",
		},
		"templates are go files": {
			ResolverConfig{OperationsFilenameTemplate: "{field}_{object}.txt"},
			"should end in .go",
		},
		"types file is a plain go file": {
			ResolverConfig{TypesFilename: "{object}.go"},
			"types_filename",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tt.cfg.Layout = LayoutFollowObject
			tt.cfg.DirName = "testdata"
			require.ErrorContains(t, tt.cfg.Check(), tt.wantErr)
		})
	}
}
