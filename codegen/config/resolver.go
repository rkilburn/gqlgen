package config

import (
	"errors"
	"fmt"
	"go/types"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type ResolverConfig struct {
	Filename            string              `yaml:"filename,omitempty"`
	FilenameTemplate    string              `yaml:"filename_template,omitempty"`
	Package             string              `yaml:"package,omitempty"`
	Type                string              `yaml:"type,omitempty"`
	Layout              ResolverLayout      `yaml:"layout,omitempty"`
	DirName             string              `yaml:"dir"`
	Batch               ResolverBatchConfig `yaml:"batch,omitempty"`
	OmitTemplateComment bool                `yaml:"omit_template_comment,omitempty"`
	ResolverTemplate    string              `yaml:"resolver_template,omitempty"`
	PreserveResolver    bool                `yaml:"preserve_resolver,omitempty"`

	// OmitResolverEmbedding declares the per-object resolver types with a named
	// "r" field instead of embedding the root resolver, which speeds up
	// compilation on large schemas.
	//
	// Enabling this is a source-breaking change for a project's own resolver
	// bodies: dependencies are reached as "r.r.myService" rather than
	// "r.myService".
	OmitResolverEmbedding bool `yaml:"omit_resolver_embedding,omitempty"`

	// The options below only apply to layout=follow-object.

	// OperationsFilenameTemplate names the file for each field of the root types (Query,
	// Mutation, Subscription). {object} and {field} are replaced with the snake_case name of the
	// type and the field. Defaults to "{field}_{object}.go", e.g. create_order_mutation.go.
	OperationsFilenameTemplate string `yaml:"operations_filename_template,omitempty"`
	// FieldsFilenameTemplate names the file for the fields of every other object. {object} is
	// replaced with the snake_case name of the object. Defaults to "{object}_fields.go", e.g.
	// order_item_fields.go. Add {field} to give each field a file of its own, e.g.
	// "{object}_{field}_resolver.go" for order_item_product_resolver.go.
	FieldsFilenameTemplate string `yaml:"fields_filename_template,omitempty"`
	// TypesFilename names the file that holds every object's resolver type and its accessor on the
	// root resolver, leaving the other files with only resolver methods. It is regenerated in full
	// each time. Defaults to "resolver_generated.go".
	TypesFilename string `yaml:"types_filename,omitempty"`
}

// ResolverBatchConfig enables batch resolver generation for resolver fields as if they
// had @goField(batch: true). Root types (Query, Mutation, Subscription), input objects,
// and introspection types (__*) are always excluded. When federation is enabled, federation
// _Service and Entity are also excluded. Global batch does not convert struct-bound fields
// into resolvers. Individual fields can opt out with @goField(batch: false).
type ResolverBatchConfig struct {
	Enabled bool
}

func (b *ResolverBatchConfig) UnmarshalYAML(unmarshal func(any) error) error {
	var enabled bool
	if err := unmarshal(&enabled); err == nil {
		b.Enabled = enabled
		return nil
	}

	var long struct {
		Enabled bool `yaml:"enabled"`
	}
	if err := unmarshal(&long); err != nil {
		return err
	}

	b.Enabled = long.Enabled
	return nil
}

type ResolverLayout string

var (
	LayoutSingleFile   ResolverLayout = "single-file"
	LayoutFollowSchema ResolverLayout = "follow-schema"
	// LayoutFollowObject writes a file per root-type field and a file per other object, with
	// the resolver types of all of them in one more file.
	LayoutFollowObject ResolverLayout = "follow-object"
)

const (
	defaultOperationsFilenameTemplate = "{field}_{object}.go"
	defaultFieldsFilenameTemplate     = "{object}_fields.go"
	defaultTypesFilename              = "resolver_generated.go"
)

var placeholderPattern = regexp.MustCompile(`\{[^{}]*\}`)

// checkPlaceholders rejects any {placeholder} in tmpl that is not one of allowed, and requires
// every one of required to be present so that two files cannot be given the same name.
func checkPlaceholders(option, tmpl string, allowed, required []string) error {
	for _, p := range placeholderPattern.FindAllString(tmpl, -1) {
		if !slices.Contains(allowed, p) {
			return fmt.Errorf(
				"%s: unknown placeholder %s in %q, expected %s",
				option, p, tmpl, strings.Join(allowed, ", "),
			)
		}
	}
	for _, p := range required {
		if !strings.Contains(tmpl, p) {
			return fmt.Errorf("%s: %q must contain %s", option, tmpl, p)
		}
	}
	if !strings.HasSuffix(tmpl, ".go") {
		return fmt.Errorf("%s: %q should end in .go", option, tmpl)
	}
	return nil
}

func (r *ResolverConfig) Check() error {
	if r.Layout == "" {
		r.Layout = LayoutSingleFile
	}
	if r.Type == "" {
		r.Type = "Resolver"
	}

	switch r.Layout {
	case LayoutSingleFile:
		if r.Filename == "" {
			return fmt.Errorf("filename must be specified with layout=%s", r.Layout)
		}
		if !strings.HasSuffix(r.Filename, ".go") {
			return fmt.Errorf(
				"filename should be path to a go source file with layout=%s",
				r.Layout,
			)
		}
		r.Filename = abs(r.Filename)
	case LayoutFollowSchema:
		if r.DirName == "" {
			return fmt.Errorf("dirname must be specified with layout=%s", r.Layout)
		}
		r.DirName = abs(r.DirName)
		if r.Filename == "" {
			r.Filename = filepath.Join(r.DirName, "resolver.go")
		} else {
			r.Filename = abs(r.Filename)
		}
	case LayoutFollowObject:
		if r.DirName == "" {
			return fmt.Errorf("dirname must be specified with layout=%s", r.Layout)
		}
		r.DirName = abs(r.DirName)
		if r.Filename == "" {
			r.Filename = filepath.Join(r.DirName, "resolver.go")
		} else {
			r.Filename = abs(r.Filename)
		}
		if r.FilenameTemplate != "" {
			return fmt.Errorf(
				"filename_template is not used with layout=%s, use operations_filename_template "+
					"and fields_filename_template",
				r.Layout,
			)
		}
		if r.OperationsFilenameTemplate == "" {
			r.OperationsFilenameTemplate = defaultOperationsFilenameTemplate
		}
		if r.FieldsFilenameTemplate == "" {
			r.FieldsFilenameTemplate = defaultFieldsFilenameTemplate
		}
		if r.TypesFilename == "" {
			r.TypesFilename = defaultTypesFilename
		}
		if err := checkPlaceholders(
			"operations_filename_template", r.OperationsFilenameTemplate,
			[]string{"{object}", "{field}"}, []string{"{field}"},
		); err != nil {
			return err
		}
		if err := checkPlaceholders(
			"fields_filename_template", r.FieldsFilenameTemplate,
			[]string{"{object}", "{field}"}, []string{"{object}"},
		); err != nil {
			return err
		}
		if strings.ContainsAny(r.TypesFilename, "{}/\\") || !strings.HasSuffix(r.TypesFilename, ".go") {
			return fmt.Errorf(
				"types_filename: %q should be a go file name with no placeholders or directories",
				r.TypesFilename,
			)
		}
	default:
		return fmt.Errorf(
			"invalid layout %s. must be %s, %s or %s",
			r.Layout,
			LayoutSingleFile,
			LayoutFollowSchema,
			LayoutFollowObject,
		)
	}

	if strings.ContainsAny(r.Package, "./\\") {
		return errors.New(
			"package should be the output package name only, do not include the output filename",
		)
	}

	if r.Package == "" && r.Dir() != "" {
		r.Package = code.NameForDir(r.Dir())
	}

	return nil
}

func (r *ResolverConfig) ImportPath() string {
	if r.Dir() == "" {
		return ""
	}
	return code.ImportPathForDir(r.Dir())
}

func (r *ResolverConfig) Dir() string {
	switch r.Layout {
	case LayoutSingleFile:
		if r.Filename == "" {
			return ""
		}
		return filepath.Dir(r.Filename)
	case LayoutFollowSchema, LayoutFollowObject:
		return r.DirName
	default:
		panic("invalid layout " + r.Layout)
	}
}

func (r *ResolverConfig) Pkg() *types.Package {
	if r.Dir() == "" {
		return nil
	}
	return types.NewPackage(r.ImportPath(), r.Package)
}

func (r *ResolverConfig) IsDefined() bool {
	return r.Filename != "" || r.DirName != ""
}
