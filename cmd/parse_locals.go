package cmd

// Terragrunt doesn't give us an easy way to access all of the Locals from a module
// in an easy to digest way. This file is mostly just follows along how Terragrunt
// parses the `locals` blocks and evaluates their contents.

import (
	"fmt"
	"github.com/gruntwork-io/go-commons/errors"
	deprecatedConfig "github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/config/hclparse"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"path/filepath"
)

// ResolvedLocals are the parsed result of local values this module cares about
type ResolvedLocals struct {
	// The Atlantis workflow to use for some project
	AtlantisWorkflow string

	// Apply requirements to override the global `--apply-requirements` flag
	ApplyRequirements []string

	// Extra dependencies that can be hardcoded in config
	ExtraAtlantisDependencies []string

	// If set, a single module will have autoplan turned to this setting
	AutoPlan *bool

	// If set to true, the module will not be included in the output
	Skip *bool

	// Terraform version to use just for this project
	TerraformVersion string

	// If set to true, create Atlantis project
	markedProject *bool
}

// parseHcl uses the HCL2 parser to parse the given string into an HCL file body.
func parseHcl(parser *hclparse.Parser, hcl string, filename string) (file *hcl.File, err error) {
	// The HCL2 parser and especially cty conversions will panic in many types of errors, so we have to recover from
	// those panics here and convert them to normal errors
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.WithStackTrace(hclparse.PanicWhileParsingConfigError{RecoveredValue: recovered, ConfigFile: filename})
		}
	}()

	if filepath.Ext(filename) == ".json" {
		file, parseDiagnostics := parser.ParseJSON([]byte(hcl), filename)
		if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
			return nil, parseDiagnostics
		}

		return file, nil
	}

	file, parseDiagnostics := parser.ParseHCL([]byte(hcl), filename)
	if parseDiagnostics != nil && parseDiagnostics.HasErrors() {
		return nil, parseDiagnostics
	}

	return file, nil
}

// Merges in values from a child into a parent set of `local` values
func mergeResolvedLocals(parent ResolvedLocals, child ResolvedLocals) ResolvedLocals {
	if child.AtlantisWorkflow != "" {
		parent.AtlantisWorkflow = child.AtlantisWorkflow
	}

	if child.TerraformVersion != "" {
		parent.TerraformVersion = child.TerraformVersion
	}

	if child.AutoPlan != nil {
		parent.AutoPlan = child.AutoPlan
	}

	if child.Skip != nil {
		parent.Skip = child.Skip
	}

	if child.markedProject != nil {
		parent.markedProject = child.markedProject
	}

	if child.ApplyRequirements != nil || len(child.ApplyRequirements) > 0 {
		parent.ApplyRequirements = child.ApplyRequirements
	}

	parent.ExtraAtlantisDependencies = append(parent.ExtraAtlantisDependencies, child.ExtraAtlantisDependencies...)

	return parent
}

// Parses a given file, returning a map of all it's `local` values
func parseLocals(ctx *TerragruntParsingContext, path string, includeFromChild *deprecatedConfig.IncludeConfig) (ResolvedLocals, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.ParsingContext.TerragruntOptions.WorkingDir, path)
	}
	// Decode just the Base blocks. See the function docs for DecodeBaseBlocks for more info on what base blocks are.
	baseBlocks, err := ctx.DecodeBaseBlocks(path, includeFromChild)
	if err != nil {
		return ResolvedLocals{}, err
	}

	// Recurse on the parent to merge in the locals from that file
	mergedParentLocals := ResolvedLocals{}
	if baseBlocks.TrackInclude != nil && includeFromChild == nil {
		for _, includeConfig := range baseBlocks.TrackInclude.CurrentList {
			parentLocals, _ := parseLocals(ctx, includeConfig.Path, &includeConfig)
			mergedParentLocals = mergeResolvedLocals(mergedParentLocals, parentLocals)
		}
	}
	childLocals, err := resolveLocals(*baseBlocks.Locals)
	if err != nil {
		return ResolvedLocals{}, err
	}
	return mergeResolvedLocals(mergedParentLocals, childLocals), nil
}

func resolveLocals(localsAsCty cty.Value) (ResolvedLocals, error) {
	resolved := ResolvedLocals{}

	// Return an empty set of locals if no `locals` block was present
	if localsAsCty == cty.NilVal {
		return resolved, nil
	}
	rawLocals := localsAsCty.AsValueMap()

	workflowValue, ok := rawLocals["atlantis_workflow"]
	if ok {
		resolved.AtlantisWorkflow = workflowValue.AsString()
	}

	versionValue, ok := rawLocals["atlantis_terraform_version"]
	if ok {
		resolved.TerraformVersion = versionValue.AsString()
	}

	autoPlanValue, ok := rawLocals["atlantis_autoplan"]
	if ok {
		hasValue := autoPlanValue.True()
		resolved.AutoPlan = &hasValue
	}

	skipValue, ok := rawLocals["atlantis_skip"]
	if ok {
		hasValue := skipValue.True()
		resolved.Skip = &hasValue
	}

	applyReqs, ok := rawLocals["atlantis_apply_requirements"]
	if ok {
		resolved.ApplyRequirements = []string{}
		it := applyReqs.ElementIterator()
		for it.Next() {
			_, val := it.Element()
			resolved.ApplyRequirements = append(resolved.ApplyRequirements, val.AsString())
		}
	}

	markedProject, ok := rawLocals["atlantis_project"]
	if ok {
		hasValue := markedProject.True()
		resolved.markedProject = &hasValue
	}

	extraDependenciesAsCty, ok := rawLocals["extra_atlantis_dependencies"]
	if ok {
		it := extraDependenciesAsCty.ElementIterator()
		for it.Next() {
			pos, val := it.Element()
			if !val.Type().Equals(cty.String) {
				posInt, _ := pos.AsBigFloat().Int64()
				return resolved, fmt.Errorf("extra_atlantis_dependencies contains non-string value at position %d", posInt)
			}

			resolved.ExtraAtlantisDependencies = append(
				resolved.ExtraAtlantisDependencies,
				filepath.ToSlash(val.AsString()),
			)
		}
	}

	return resolved, nil
}
