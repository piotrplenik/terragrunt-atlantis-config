package cmd

import (
	"context"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/config/hclparse"
	"github.com/gruntwork-io/terragrunt/options"
	"github.com/gruntwork-io/terragrunt/pkg/log"
	"github.com/gruntwork-io/terragrunt/pkg/log/format"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/hashicorp/hcl/v2"
	"os"
	"path/filepath"
	"strings"
	_ "unsafe"
)

type parsedHcl struct {
	Terraform *config.TerraformConfig `hcl:"terraform,block"`
	Includes  []config.IncludeConfig  `hcl:"include,block"`
}

// terragruntIncludeMultiple is a struct that can be used to only decode the include block with labels.
type terragruntIncludeMultiple struct {
	Include []config.IncludeConfig `hcl:"include,block"`
	Remain  hcl.Body               `hcl:",remain"`
}

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

// createLogger creates a logger with proper formatter to avoid nil pointer dereference
func createLogger() log.Logger {
	formatter := format.NewFormatter(format.NewKeyValueFormatPlaceholders())
	formatter.SetDisabledColors(true)
	return log.New(log.WithLevel(log.ErrorLevel), log.WithFormatter(formatter))
}

func NewParsingContextWithConfigPath(ctx context.Context, terragruntConfigPath string) (*TerragruntParsingContext, error) {
	opt, err := options.NewTerragruntOptionsWithConfigPath(terragruntConfigPath)
	if err != nil {
		return nil, err
	}
	opt.OriginalTerragruntConfigPath = terragruntConfigPath
	opt.Env = getEnvs()
	
	// Create logger with proper formatter
	logger := createLogger()
	
	// Attach logger to context
	ctx = log.ContextWithLogger(ctx, logger)

	parsingContext := config.NewParsingContext(ctx, logger, opt)

	terragruntParsingContext := TerragruntParsingContext{
		Context:        ctx,
		ParsingContext: parsingContext,
	}

	return &terragruntParsingContext, nil
}

func NewParsingContextWithDecodeList(ctx *TerragruntParsingContext) *TerragruntParsingContext {
	// Create logger with proper formatter
	logger := createLogger()
	
	// Ensure the context has a logger attached
	contextWithLogger := log.ContextWithLogger(ctx.ParsingContext.Context, logger)
	
	// Parse the HCL file
	parseCtx := config.NewParsingContext(contextWithLogger, logger, ctx.ParsingContext.TerragruntOptions).
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
	// Create logger with proper formatter
	logger := createLogger()
	
	parseConfig, err := config.PartialParseConfigFile(ctx.ParsingContext, logger, path, nil)
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
	
	// Create logger with proper formatter
	logger := createLogger()
	
	// Ensure the context has a logger attached
	contextWithLogger := log.ContextWithLogger(ctx.Context, logger)

	terrContext := config.NewParsingContext(contextWithLogger, logger, terrOpts)

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
	
	// Create logger with proper formatter
	logger := createLogger()
	
	return config.DecodeBaseBlocks(parsingContext, logger, file, includeFromChild)
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

//go:linkname createTerragruntEvalContext github.com/gruntwork-io/terragrunt/config.createTerragruntEvalContext
func createTerragruntEvalContext(ctx *config.ParsingContext, l log.Logger, configPath string) (*hcl.EvalContext, error)
