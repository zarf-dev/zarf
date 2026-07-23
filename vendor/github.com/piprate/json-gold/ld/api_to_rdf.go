// Copyright 2015-2017 Piprate Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ld

// ToRDF adds RDF triples for each graph in the current node map to an RDF dataset.
func (api *JsonLdApi) ToRDF(input interface{}, opts *JsonLdOptions) (*RDFDataset, error) {
	issuer := NewIdentifierIssuer("_:b")

	nodeMap := make(map[string]interface{})
	nodeMap["@default"] = make(map[string]interface{})
	if _, err := api.GenerateNodeMap(input, nodeMap, "@default", issuer, "", "", nil); err != nil {
		return nil, err
	}

	dataset := NewRDFDataset()

	for graphName, graphVal := range nodeMap {
		// 4.1)
		if IsRelativeIri(graphName) {
			continue
		}
		graph := graphVal.(map[string]interface{})
		dataset.GraphToRDF(graphName, graph, issuer, opts.ProduceGeneralizedRdf)
	}

	return dataset, nil
}
