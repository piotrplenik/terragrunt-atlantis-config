package cmd

import (
	"context"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/config/hclparse"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/util"
	"os"
	"path/filepath"
	"strings"
)

type TerragruntParsingContext struct {
	context.Context

	ParsingContext *config.ParsingContext
}

type IntegrationTerragruntConfig struct {
	*config.TerragruntConfig
}

// Parse env vars into a map
func getEnvs() map[string]string {
	envs := os.Environ()
	m := make(map[string]string)

	for _, env := range envs {
		results := strings.SplitN(env, "=", 2)
		m[results[0]] = results[1]
	}

	return m
}

func NewParsingContextWithConfigPath(ctx context.Context, terragruntConfigPath string) (*TerragruntParsingContext, error) {
	opt, err := options.NewTerragruntOptionsWithConfigPath(terragruntConfigPath)
	if err != nil {
		return nil, err
	}
	opt.OriginalTerragruntConfigPath = terragruntConfigPath
	opt.Env = getEnvs()

	parsingContext := config.NewParsingContext(ctx, opt)

	terragruntParsingContext := TerragruntParsingContext{
		Context:        ctx,
		ParsingContext: parsingContext,
	}

	return &terragruntParsingContext, nil
}

func NewParsingContextWithDecodeList(ctx *TerragruntParsingContext) *TerragruntParsingContext {
	// Parse the HCL file
	parseCtx := config.NewParsingContext(ctx.ParsingContext, ctx.ParsingContext.TerragruntOptions).
		WithDecodeList(
			config.DependencyBlock,
			config.TerraformBlock,
		)

	terragruntParsingContext := TerragruntParsingContext{
		Context:        ctx.Context,
		ParsingContext: parseCtx,
	}

	return &terragruntParsingContext
}

func (ctx TerragruntParsingContext) WithDecodedList() *TerragruntParsingContext {
	ctx.ParsingContext.WithDecodeList(
		config.DependencyBlock,
		config.DependenciesBlock,
		config.TerraformBlock,
	)

	return &ctx
}

func (ctx TerragruntParsingContext) WithTerragruntOptions(opts *options.TerragruntOptions) *TerragruntParsingContext {
	ctx.ParsingContext.WithTerragruntOptions(opts)

	return &ctx
}

func (ctx TerragruntParsingContext) PartialParseConfigFile(path string) (*IntegrationTerragruntConfig, error) {
	parseConfig, err := config.PartialParseConfigFile(ctx.ParsingContext, path, nil)
	if err != nil {
		return nil, err
	}
	terragruntIntegrationConfig := IntegrationTerragruntConfig{
		TerragruntConfig: parseConfig,
	}
	return &terragruntIntegrationConfig, nil
}

func (ctx TerragruntParsingContext) WithDependencyPath(path string) *TerragruntParsingContext {
	terrOpts, _ := options.NewTerragruntOptionsWithConfigPath(path)
	terrOpts.OriginalTerragruntConfigPath = ctx.ParsingContext.TerragruntOptions.OriginalTerragruntConfigPath
	terrOpts.Env = ctx.ParsingContext.TerragruntOptions.Env
	terrContext := config.NewParsingContext(ctx, terrOpts)

	terragruntParsingContext := TerragruntParsingContext{
		Context:        ctx.Context,
		ParsingContext: terrContext,
	}

	return &terragruntParsingContext
}

// DecodeBaseBlocks Decode just the Base blocks. See the function docs for DecodeBaseBlocks for more info on what base blocks are.
func (ctx TerragruntParsingContext) DecodeBaseBlocks(path string, includeFromChild *config.IncludeConfig) (*config.DecodedBaseBlocks, error) {
	parsingContext := ctx.ParsingContext.
		WithDecodeList(config.DependencyBlock, config.DependenciesBlock, config.TerraformBlock)

	file, err := hclparse.NewParser(ctx.ParsingContext.ParserOptions...).ParseFromFile(path)
	if err != nil {
		return nil, err
	}
	return config.DecodeBaseBlocks(parsingContext, file, includeFromChild)
}

// FindConfigFilesInPath returns a list of all Terragrunt config files in the given path or any subfolder of the path. A file is a Terragrunt
// config file if it has a name as returned by the DefaultConfigPath method
func FindConfigFilesInPath(rootPath string, opts *options.TerragruntOptions) ([]string, error) {
	configFiles := []string{}

	walkFunc := filepath.Walk

	err := walkFunc(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		for _, configFile := range []string{"root.hcl"} {
			if !filepath.IsAbs(configFile) {
				configFile = util.JoinPath(path, configFile)
			}

			if !util.IsDir(configFile) && util.FileExists(configFile) {
				configFiles = append(configFiles, configFile)
				break
			}
		}

		return nil
	})

	nestedConfigFiles, err := config.FindConfigFilesInPath(rootPath, opts)
	if err == nil {
		configFiles = append(configFiles, nestedConfigFiles...)
	}
	return configFiles, nil
}

// Finds the absolute paths of all terragrunt.hcl files
func getAllTerragruntFiles(path string) ([]string, error) {
	terragruntOptions, err := options.NewTerragruntOptionsWithConfigPath(path)
	if err != nil {
		return nil, err
	}

	// If filterPaths is provided, override workingPath instead of gitRoot
	// We do this here because we want to keep the relative path structure of Terragrunt files
	// to root and just ignore the ConfigFiles
	workingPaths := []string{path}

	// filters are not working (yet) if using project hcl files (which are kind of filters by themselves)
	if len(filterPaths) > 0 && len(projectHclFiles) == 0 {
		workingPaths = []string{}
		for _, filterPath := range filterPaths {
			// get all matching folders
			theseWorkingPaths, err := filepath.Glob(filterPath)
			if err != nil {
				return nil, err
			}
			workingPaths = append(workingPaths, theseWorkingPaths...)
		}
	}

	uniqueConfigFilePaths := make(map[string]bool)
	orderedConfigFilePaths := []string{}
	for _, workingPath := range workingPaths {
		paths, err := FindConfigFilesInPath(workingPath, terragruntOptions)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			// if path not yet seen, insert once
			if !uniqueConfigFilePaths[p] {
				orderedConfigFilePaths = append(orderedConfigFilePaths, p)
				uniqueConfigFilePaths[p] = true
			}
		}
	}

	uniqueConfigFileAbsPaths := []string{}
	for _, uniquePath := range orderedConfigFilePaths {
		uniqueAbsPath, err := filepath.Abs(uniquePath)
		if err != nil {
			return nil, err
		}
		uniqueConfigFileAbsPaths = append(uniqueConfigFileAbsPaths, uniqueAbsPath)
	}

	return uniqueConfigFileAbsPaths, nil
}
