// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

// GenerateValuesSchemaOptions configures schema generation from a package or component config.
type GenerateValuesSchemaOptions struct {
	load.DefinitionOptions
	// Update writes the schema and adds its reference to the source definition when absent.
	Update bool
	// DeleteNotFound removes existing schema properties absent from the generated schema.
	DeleteNotFound bool
}

// GenerateValuesSchema generates a values schema from defaults, rendered chart values files,
// and chart schemas, reconciled with any existing values schema.
func GenerateValuesSchema(ctx context.Context, source string, opts GenerateValuesSchemaOptions) (value.SchemaDocument, error) {
	l := logger.From(ctx)
	packagePath, err := layout.ResolvePackagePath(source)
	if err != nil {
		return nil, err
	}
	loaded, err := loadDefinition(ctx, packagePath.ManifestFile, load.PackageOptions{
		DefinitionOptions:          opts.DefinitionOptions,
		SkipValuesSchemaValidation: true,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := loaded.Close(); closeErr != nil {
			l.Warn("unable to close loaded package", "error", closeErr)
		}
	}()
	pkg := loaded.Definition

	variableConfig, err := getPopulatedVariableConfig(ctx, pkg, nil, opts.IsInteractive)
	if err != nil {
		return nil, err
	}
	s, err := state.Default()
	if err != nil {
		return nil, err
	}

	// Step 1: Copy package defaults
	packageValues := loaded.Values.DeepCopy()

	var mappedSchemas []map[string]any
	var mappedInferredSchemas []map[string]any

	// Step 2: Collect chart value schemas
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			l.Warn("unable to clean up temporary directory", "path", tmpDir, "error", err.Error())
		}
	}()

	for _, component := range pkg.Components {
		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, s)
		if err != nil {
			return nil, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)
		for _, chart := range component.Charts {
			chartPaths := layout.ChartPaths{
				ChartsDir: filepath.Join(tmpDir, component.Name, "charts", chart.Name),
				ValuesDir: filepath.Join(tmpDir, component.Name, "values"),
			}

			err := assemble.PackageChart(ctx, chart, loaded.Resources, chartPaths, opts.CachePath, opts.RemoteOptions)
			if err != nil {
				return nil, fmt.Errorf("unable to package chart %q for schema generation: %w", chart.Name, err)
			}

			if err := templateValuesFiles(ctx, chart, chartPaths.ValuesDir, templateValuesFilesOpts{
				variableConfig: variableConfig,
				pkg:            pkg,
				vals:           packageValues,
				s:              s,
				stateAccess:    component.StateAccess,
			}); err != nil {
				return nil, fmt.Errorf("unable to template values files for chart %q: %w", chart.Name, err)
			}

			helmChart, valuesFilesValues, err := helm.LoadChartData(chart, chartPaths, nil)
			if err != nil {
				return nil, fmt.Errorf("unable to load default values for chart %q: %w", chart.Name, err)
			}

			appliedValues := helpers.MergeMapRecursive(helmChart.Values, valuesFilesValues)
			inferredSchema := value.GenerateJSONSchema(value.Values(helmChart.Values))
			valuesFilesSchema := value.GenerateJSONSchema(value.Values(valuesFilesValues))
			if err := value.MergeJSONSchemaAtPath(inferredSchema, value.Path("."), valuesFilesSchema); err != nil {
				return nil, fmt.Errorf("unable to apply valuesFiles for chart %q: %w", chart.Name, err)
			}
			var chartSchema map[string]any
			if len(helmChart.Schema) > 0 {
				if err := json.Unmarshal(helmChart.Schema, &chartSchema); err != nil {
					l.Warn("unable to parse Helm chart values schema; falling back to inferred types", "chart", chart.Name, "error", err)
					chartSchema = nil
				} else {
					chartSchema = value.FilterChartSchema(chartSchema)
					if chartSchema != nil {
						if err := value.ValidateSchemaDocument(chartSchema); err != nil {
							l.Warn("unable to validate Helm chart values schema; falling back to inferred types", "chart", chart.Name, "error", err)
							chartSchema = nil
						}
					}
				}
			}

			// Map chart schemas from target paths to package value source paths.
			for _, cv := range chart.Values {
				if cv.SourcePath == "" || cv.TargetPath == "" {
					return nil, fmt.Errorf("chart %q value mapping has empty sourcePath or targetPath", chart.Name)
				}
				mappedValue, err := value.Values(appliedValues).Extract(value.Path(cv.TargetPath))
				if err != nil {
					return nil, fmt.Errorf("unable to extract chart %q value at targetPath %q: %w", chart.Name, cv.TargetPath, err)
				}
				targetInferredSchema, found, err := value.ExtractJSONSchema(inferredSchema, value.Path(cv.TargetPath))
				if err != nil {
					return nil, fmt.Errorf("unable to inspect chart %q values at targetPath %q: %w", chart.Name, cv.TargetPath, err)
				}
				if found {
					mappedValues := value.Values{}
					if err := mappedValues.Set(value.Path(cv.SourcePath), mappedValue); err != nil {
						return nil, fmt.Errorf("unable to set chart %q value at sourcePath %q: %w", chart.Name, cv.SourcePath, err)
					}
					mappedSchema := value.GenerateJSONSchema(mappedValues)
					mappedSchema, err = mapSchemaToSource(mappedSchema, targetInferredSchema, value.Path(cv.SourcePath), cv.ExcludePaths)
					if err != nil {
						return nil, fmt.Errorf("unable to map inferred schema for chart %q: %w", chart.Name, err)
					}
					mappedInferredSchemas = append(mappedInferredSchemas, mappedSchema)
				}

				if chartSchema != nil {
					targetSchema, found, err := value.ExtractJSONSchema(chartSchema, value.Path(cv.TargetPath))
					if err != nil {
						return nil, fmt.Errorf("unable to inspect chart %q schema at targetPath %q: %w", chart.Name, cv.TargetPath, err)
					}
					if found {
						mappedSchema, err := mapSchemaToSource(nil, targetSchema, value.Path(cv.SourcePath), cv.ExcludePaths)
						if err != nil {
							return nil, fmt.Errorf("unable to map Helm schema for chart %q: %w", chart.Name, err)
						}
						mappedSchemas = append(mappedSchemas, mappedSchema)
					} else {
						l.Warn("chart values schema does not define mapped target; falling back to inferred types", "chart", chart.Name, "targetPath", cv.TargetPath)
					}
				}
			}
		}
	}

	// Step 3: Merge inferred and chart schemas
	generatedSchema := value.GenerateJSONSchema(value.Values{})
	for _, mapped := range mappedInferredSchemas {
		if err := value.MergeGeneratedJSONSchemaAtPath(generatedSchema, value.Path("."), mapped); err != nil {
			return nil, fmt.Errorf("unable to apply inferred chart values: %w", err)
		}
	}
	if err := value.MergeGeneratedJSONSchemaAtPath(generatedSchema, value.Path("."), value.GenerateJSONSchema(packageValues)); err != nil {
		return nil, fmt.Errorf("unable to apply package defaults: %w", err)
	}

	for _, mapped := range mappedSchemas {
		if err := value.MergeGeneratedJSONSchemaAtPath(generatedSchema, value.Path("."), mapped); err != nil {
			return nil, fmt.Errorf("unable to apply Helm schema: %w", err)
		}
	}

	// Step 4: Merge and reconcile any existing schema
	existingSchema := loaded.ValuesSchema

	if existingSchema != nil {
		generatedSchema = value.ReconcileJSONSchema(existingSchema, generatedSchema, opts.DeleteNotFound)
	}

	// Step 5: Save the resulting schema when requested.
	if opts.Update {
		b, err := json.MarshalIndent(generatedSchema, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("unable to marshal schema to JSON: %w", err)
		}
		schemaPath := pkg.Values.Schema
		if schemaPath == "" {
			schemaPath = "values.schema.json"
		}
		outputFileName := schemaPath
		if !filepath.IsAbs(outputFileName) {
			outputFileName = filepath.Join(packagePath.BaseDir, outputFileName)
		}

		if err := os.WriteFile(outputFileName, b, helpers.ReadAllWriteUser); err != nil {
			return nil, fmt.Errorf("unable to write schema file: %w", err)
		}
		if pkg.Values.Schema == "" {
			if err := UpdateSchema(ctx, packagePath.ManifestFile, schemaPath); err != nil {
				return nil, fmt.Errorf("unable to update package definition with schema path: %w", err)
			}
		}

		l.Info("Schema successfully generated", "filename", outputFileName)
	}
	return generatedSchema, nil
}

func mapSchemaToSource(mappedSchema, schema map[string]any, sourcePath value.Path, excludePaths []string) (map[string]any, error) {
	if mappedSchema == nil {
		mappedValues := value.Values{}
		if sourcePath != "." {
			if err := mappedValues.Set(sourcePath, nil); err != nil {
				return nil, fmt.Errorf("unable to set sourcePath %q: %w", sourcePath, err)
			}
		}
		mappedSchema = value.GenerateJSONSchema(mappedValues)
	}

	if err := value.MergeJSONSchemaAtPath(mappedSchema, sourcePath, schema); err != nil {
		return nil, fmt.Errorf("unable to apply schema at sourcePath %q: %w", sourcePath, err)
	}
	for _, excludePath := range excludePaths {
		if err := value.DeleteJSONSchemaAtPath(mappedSchema, value.Path(excludePath)); err != nil {
			return nil, fmt.Errorf("unable to exclude schema path %q: %w", excludePath, err)
		}
	}
	return mappedSchema, nil
}
