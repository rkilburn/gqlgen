package config

import (
	"errors"
	"fmt"
	"go/types"
	"path/filepath"
	"regexp"
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

	// MutationFilenameTemplate, with layout=follow-schema, gives each field of the Mutation type
	// a file of its own instead of the file for its schema file. {field} is replaced with the
	// snake_case name of the field and {name} with the name of the schema file declaring it, e.g.
	// "{field}_mutation.go" gives create_order_mutation.go.
	MutationFilenameTemplate string `yaml:"mutation_filename_template,omitempty"`
	// SubscriptionFilenameTemplate does the same for the fields of the Subscription type, e.g.
	// "{field}_subscription.go" gives order_sent_subscription.go.
	SubscriptionFilenameTemplate string `yaml:"subscription_filename_template,omitempty"`
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
)

var placeholderPattern = regexp.MustCompile(`\{[^{}]*\}`)

// checkOperationFilenameTemplate validates a mutation or subscription filename template, if one is
// set. It requires {field}, so that two fields cannot share a file name, and rejects any other
// placeholder than {name}.
func checkOperationFilenameTemplate(option, tmpl string) error {
	if tmpl == "" {
		return nil
	}
	for _, p := range placeholderPattern.FindAllString(tmpl, -1) {
		if p != "{field}" && p != "{name}" {
			return fmt.Errorf(
				"%s: unknown placeholder %s in %q, expected {field} or {name}", option, p, tmpl,
			)
		}
	}
	if !strings.Contains(tmpl, "{field}") {
		return fmt.Errorf("%s: %q must contain {field}", option, tmpl)
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
		if r.MutationFilenameTemplate != "" || r.SubscriptionFilenameTemplate != "" {
			return fmt.Errorf(
				"mutation_filename_template and subscription_filename_template need layout=%s, not %s",
				LayoutFollowSchema,
				r.Layout,
			)
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
		if err := checkOperationFilenameTemplate(
			"mutation_filename_template", r.MutationFilenameTemplate,
		); err != nil {
			return err
		}
		if err := checkOperationFilenameTemplate(
			"subscription_filename_template", r.SubscriptionFilenameTemplate,
		); err != nil {
			return err
		}
		r.DirName = abs(r.DirName)
		if r.Filename == "" {
			r.Filename = filepath.Join(r.DirName, "resolver.go")
		} else {
			r.Filename = abs(r.Filename)
		}
	default:
		return fmt.Errorf(
			"invalid layout %s. must be %s or %s",
			r.Layout,
			LayoutSingleFile,
			LayoutFollowSchema,
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
	case LayoutFollowSchema:
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
