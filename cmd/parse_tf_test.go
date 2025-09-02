package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTerraformLocalModuleSource_TofuFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "tofu-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory for a local module
	moduleDir := filepath.Join(tmpDir, "local-module")
	err = os.MkdirAll(moduleDir, 0755)
	require.NoError(t, err)

	// Create a .tofu file with module calls
	tofuContent := `
module "vpc" {
  source = "./modules/vpc"
  
  cidr = "10.0.0.0/16"
}

module "eks" {
  source = "../shared/eks"
  
  vpc_id = module.vpc.id
}

module "remote" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 3.0"
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.tofu"), []byte(tofuContent), 0644)
	require.NoError(t, err)

	// Create a .tofu.json file with module calls
	tofuJsonContent := `{
  "module": {
    "database": {
      "source": "./modules/database",
      "instance_type": "db.t3.micro"
    },
    "monitoring": {
      "source": "../shared/monitoring",
      "enabled": true
    }
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "database.tofu.json"), []byte(tofuJsonContent), 0644)
	require.NoError(t, err)

	// Create some dummy module files to make the paths valid
	vpcModuleDir := filepath.Join(tmpDir, "modules", "vpc")
	err = os.MkdirAll(vpcModuleDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(vpcModuleDir, "main.tofu"), []byte("# VPC module"), 0644)
	require.NoError(t, err)

	dbModuleDir := filepath.Join(tmpDir, "modules", "database")
	err = os.MkdirAll(dbModuleDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dbModuleDir, "main.tofu"), []byte("# Database module"), 0644)
	require.NoError(t, err)

	sharedEksDir := filepath.Join(filepath.Dir(tmpDir), "shared", "eks")
	err = os.MkdirAll(sharedEksDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sharedEksDir, "main.tofu"), []byte("# EKS module"), 0644)
	require.NoError(t, err)

	sharedMonitoringDir := filepath.Join(filepath.Dir(tmpDir), "shared", "monitoring")
	err = os.MkdirAll(sharedMonitoringDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sharedMonitoringDir, "main.tofu"), []byte("# Monitoring module"), 0644)
	require.NoError(t, err)

	// Test the function
	sources, err := parseTerraformLocalModuleSource(tmpDir)
	require.NoError(t, err)

	// Verify the results - should find both *.tf* and *.tofu* patterns for each local module
	assert.Len(t, sources, 8, "Should find 8 glob patterns (4 modules × 2 patterns each)")

	expectedModules := []string{
		filepath.Join(tmpDir, "modules", "vpc"),
		filepath.Join(tmpDir, "modules", "database"),
		filepath.Join(sharedEksDir),
		filepath.Join(sharedMonitoringDir),
	}

	// Check that each module has both *.tf* and *.tofu* patterns
	for _, moduleDir := range expectedModules {
		tfPattern := filepath.Join(moduleDir, "*.tf*")
		tofuPattern := filepath.Join(moduleDir, "*.tofu*")
		
		assert.Contains(t, sources, tfPattern, "Should contain %s", tfPattern)
		assert.Contains(t, sources, tofuPattern, "Should contain %s", tofuPattern)
	}

	// Verify that remote modules are not included
	for _, source := range sources {
		assert.NotContains(t, source, "terraform-aws-modules", "Should not include remote modules")
	}
}

func TestExtractModuleCallSources_TofuFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "tofu-extract-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a .tofu file
	tofuContent := `
module "local_module" {
  source = "./local"
}

module "relative_module" {
  source = "../relative"
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "test.tofu"), []byte(tofuContent), 0644)
	require.NoError(t, err)

	// Create a .tf file for comparison
	err = os.WriteFile(filepath.Join(tmpDir, "test.tf"), []byte(tofuContent), 0644)
	require.NoError(t, err)

	// Create a .tofu.json file
	tofuJsonContent := `{
  "module": {
    "json_module": {
      "source": "./json-module"
    }
  }
}`
	err = os.WriteFile(filepath.Join(tmpDir, "test.tofu.json"), []byte(tofuJsonContent), 0644)
	require.NoError(t, err)

	// Test the function
	sources, err := extractModuleCallSources(tmpDir)
	require.NoError(t, err)

	// Should find sources from all files
	t.Logf("Found sources: %v", sources)
	
	// Should include sources from .tofu, .tf, and .tofu.json files
	expectedSources := []string{"./local", "../relative", "./json-module"}
	
	// Since we have both .tf and .tofu with same content, we'll get duplicates
	// Let's just check that all expected sources are found
	for _, expected := range expectedSources {
		found := false
		for _, source := range sources {
			if source == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find source: %s", expected)
	}
}

func TestIsLocalTerraformModuleSource(t *testing.T) {
	testCases := []struct {
		source   string
		expected bool
	}{
		{"./local", true},
		{"../parent", true},
		{".\\windows", true},
		{"..\\windows-parent", true},
		{"terraform-aws-modules/vpc/aws", false},
		{"git::https://github.com/user/repo.git", false},
		{"s3::https://bucket.s3.amazonaws.com/module.zip", false},
		{"./relative/path", true},
		{"../../../deep/relative", true},
	}

	for _, tc := range testCases {
		t.Run(tc.source, func(t *testing.T) {
			result := isLocalTerraformModuleSource(tc.source)
			assert.Equal(t, tc.expected, result, "isLocalTerraformModuleSource(%q) should return %v", tc.source, tc.expected)
		})
	}
}
