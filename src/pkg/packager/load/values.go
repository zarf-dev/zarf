// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/value"
)

func loadValues(ctx context.Context, resources *ResourceSet, plan valuePlan, skipSchemaValidation bool) (value.Values, value.SchemaDocument, error) {
	paths := make([]string, 0, len(plan.files))
	for _, source := range plan.files {
		physical, err := resources.Path(source)
		if err != nil {
			return nil, nil, err
		}
		paths = append(paths, physical)
	}

	values := value.Values{}
	if len(paths) > 0 {
		var err error
		values, err = value.ParseFiles(ctx, paths, value.ParseFilesOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse values files: %w", err)
		}
	}

	schemas := make([]value.SchemaDocument, 0, len(plan.schemas))
	for _, source := range plan.schemas {
		contents, err := resources.ReadFile(source)
		if err != nil {
			return nil, nil, fmt.Errorf("reading values schema %q: %w", source, err)
		}
		schema, err := value.ParseSchemaDocument(source, contents)
		if err != nil {
			return nil, nil, err
		}
		schemas = append(schemas, schema)
	}
	merged, err := value.MergeSchemaDocuments(schemas)
	if err != nil {
		return nil, nil, err
	}
	if merged != nil && !skipSchemaValidation {
		if err := values.ValidateAgainstSchema(ctx, merged, "resolved values schema", value.ValidateOptions{SkipRequired: true}); err != nil {
			return nil, nil, fmt.Errorf("values validation failed: %w", err)
		}
	}
	return values, merged, nil
}
