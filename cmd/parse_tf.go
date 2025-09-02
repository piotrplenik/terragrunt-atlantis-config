package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

var localModuleSourcePrefixes = []string{
	"./",
	"../",
	".\\",
	"..\\",
}

func parseTerraformLocalModuleSource(path string) ([]string, error) {
	moduleCallSources, err := extractModuleCallSources(path)
	if err != nil {
		return nil, err
	}

	var sourceMap = map[string]bool{}
	for _, source := range moduleCallSources {
		if isLocalTerraformModuleSource(source) {
			modulePath := util.JoinPath(path, source)
			modulePathGlob := util.JoinPath(modulePath, "*.tf*")

			if _, exists := sourceMap[modulePathGlob]; exists {
				continue
			}
			sourceMap[modulePathGlob] = true

			// find local module source recursively
			subSources, err := parseTerraformLocalModuleSource(modulePath)
			if err != nil {
				return nil, err
			}

			for _, subSource := range subSources {
				sourceMap[subSource] = true
			}
		}
	}

	var sources = []string{}
	for source := range sourceMap {
		sources = append(sources, source)
	}

	return sources, nil
}

// extractModuleCallSources parses HCL files in a directory and extracts module call sources
func extractModuleCallSources(dir string) ([]string, error) {
	var sources []string

	// Find all .tf and .tf.json files
	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}
	jsonFiles, err := filepath.Glob(filepath.Join(dir, "*.tf.json"))
	if err != nil {
		return nil, err
	}
	files = append(files, jsonFiles...)

	parser := hclparse.NewParser()

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue // Skip files we can't read
		}

		var f *hcl.File
		var diags hcl.Diagnostics

		if strings.HasSuffix(file, ".json") {
			f, diags = parser.ParseJSON(content, file)
		} else {
			f, diags = parser.ParseHCL(content, file)
		}

		if diags.HasErrors() {
			continue // Skip files with parse errors
		}

		// Extract module calls from the parsed file
		fileSources := extractModuleCallsFromFile(f)
		sources = append(sources, fileSources...)
	}

	return sources, nil
}

// extractModuleCallsFromFile extracts module call sources from a parsed HCL file
func extractModuleCallsFromFile(file *hcl.File) []string {
	var sources []string

	// Handle HCL native syntax
	if body, ok := file.Body.(*hclsyntax.Body); ok {
		for _, block := range body.Blocks {
			if block.Type == "module" && len(block.Labels) > 0 {
				// Look for the source attribute
				if sourceAttr, exists := block.Body.Attributes["source"]; exists {
					if sourceValue, diags := sourceAttr.Expr.Value(nil); !diags.HasErrors() {
						if sourceValue.Type().Equals(cty.String) && sourceValue.IsKnown() && !sourceValue.IsNull() {
							sources = append(sources, sourceValue.AsString())
						}
					}
				}
			}
		}
	}

	return sources
}

func isLocalTerraformModuleSource(raw string) bool {
	for _, prefix := range localModuleSourcePrefixes {
		if strings.HasPrefix(raw, prefix) {
			return true
		}
	}

	return false
}
