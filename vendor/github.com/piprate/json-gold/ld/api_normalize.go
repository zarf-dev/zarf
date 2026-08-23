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

import (
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	hashPkg "hash"
	"sort"
	"strings"
)

const (
	AlgorithmURDNA2015 = "URDNA2015"
	AlgorithmURGNA2012 = "URGNA2012"
)

func (api *JsonLdApi) Normalize(dataset *RDFDataset, opts *JsonLdOptions) (interface{}, error) {
	algo := NewNormalisationAlgorithm(opts.Algorithm)
	return algo.Main(dataset, opts)
}

var (
	Positions = []string{"s", "o", "g"}
)

type NormalisationAlgorithm struct {
	blankNodeInfo    map[string]map[string]interface{}
	hashToBlankNodes map[string][]string
	canonicalIssuer  *IdentifierIssuer
	quads            []*Quad
	lines            []string
	version          string
}

func NewNormalisationAlgorithm(version string) *NormalisationAlgorithm {
	return &NormalisationAlgorithm{
		blankNodeInfo:   make(map[string]map[string]interface{}),
		canonicalIssuer: NewIdentifierIssuer("_:c14n"),
		quads:           make([]*Quad, 0),
		version:         version,
	}
}

func (na *NormalisationAlgorithm) Quads() []*Quad {
	return na.quads
}

func (na *NormalisationAlgorithm) Normalize(dataset *RDFDataset) {
	// 1) Create the normalisation state

	// 2) For every quad in input dataset:
	for graphName, triples := range dataset.Graphs {
		if graphName == "@default" {
			graphName = ""
		}
		for _, quad := range triples {
			if graphName != "" {
				if strings.Index(graphName, "_:") == 0 {
					quad.Graph = NewBlankNode(graphName)
				} else {
					quad.Graph = NewIRI(graphName)
				}
			}

			na.quads = append(na.quads, quad)

			// 2.1) For each blank node that occurs in the quad, add
			// a reference to the quad using the blank node identifier
			// in the blank node to quads map, creating a new entry if necessary.
			for _, attrNode := range []Node{quad.Subject, quad.Object, quad.Graph} {
				if attrNode != nil {
					if IsBlankNode(attrNode) {
						id := attrNode.GetValue()
						bNodeInfo, hasID := na.blankNodeInfo[id]
						if !hasID {
							bNodeInfo = map[string]interface{}{
								"quads": make([]*Quad, 0),
							}
							na.blankNodeInfo[id] = bNodeInfo
						}
						bNodeInfo["quads"] = append(bNodeInfo["quads"].([]*Quad), quad)
					}
				}
			}
		}
	}

	// 3) Create a list of non-normalized blank node identifiers and
	// populate it using the keys from the blank node to quads map.
	nonNormalized := make(map[string]bool)
	for id := range na.blankNodeInfo {
		nonNormalized[id] = true
	}

	// 4) Initialize simple, a boolean flag, to true.
	simple := true

	// 5) While simple is true, issue canonical identifiers for blank nodes:
	for simple {
		// 5.1) Set simple to false.
		simple = false

		// 5.2) Clear hash to blank nodes map.
		na.hashToBlankNodes = make(map[string][]string)

		// 5.3) For each blank node identifier in non-normalized identifiers:
		for id := range nonNormalized {
			// 5.3.1) Create a hash, hash, according to the Hash First Degree Quads algorithm.
			hash := na.hashFirstDegreeQuads(id)

			// 5.3.2) Add hash and identifier to hash to blank nodes map,
			// creating a new entry if necessary.
			bNodeList, hasList := na.hashToBlankNodes[hash]
			if !hasList {
				bNodeList = make([]string, 0)

			}
			na.hashToBlankNodes[hash] = append(bNodeList, id)
		}

		// 5.4) For each hash to identifier list mapping in hash to blank
		// nodes map, lexicographically-sorted by hash:
		sortedHashes := make([]string, len(na.hashToBlankNodes))
		i := 0
		for key := range na.hashToBlankNodes {
			sortedHashes[i] = key
			i++
		}
		sort.Strings(sortedHashes)
		for _, hash := range sortedHashes {
			idList := na.hashToBlankNodes[hash]
			// 5.4.1) If the length of identifier list is greater than 1,
			// continue to the next mapping.
			if len(idList) > 1 {
				continue
			}

			// 5.4.2) Use the Issue Identifier algorithm, passing canonical
			// issuer and the single blank node identifier in identifier
			// list, identifier, to issue a canonical replacement identifier
			// for identifier.
			id := idList[0]
			na.canonicalIssuer.GetId(id)

			// 5.4.3) Remove identifier from non-normalized identifiers.
			delete(nonNormalized, id)

			// 5.4.4) Remove hash from the hash to blank nodes map.
			delete(na.hashToBlankNodes, hash)

			// 5.4.5) Set simple to true.
			simple = true
		}
	}

	// 6) For each hash to identifier list mapping in hash to blank nodes
	// map, lexicographically-sorted by hash:
	sortedHashes := make([]string, len(na.hashToBlankNodes))
	i := 0
	for key := range na.hashToBlankNodes {
		sortedHashes[i] = key
		i++
	}
	sort.Strings(sortedHashes)
	for _, hash := range sortedHashes {
		idList := na.hashToBlankNodes[hash]
		// 6.1) Create hash path list where each item will be a result of
		// running the Hash N-Degree Quads algorithm.
		hashPaths := make(map[string][]*IdentifierIssuer)

		// 6.2) For each blank node identifier identifier in identifier list:
		for _, id := range idList {
			// 6.2.1) If a canonical identifier has already been issued for
			// identifier, continue to the next identifier.
			if na.canonicalIssuer.HasId(id) {
				continue
			}

			// 6.2.2) Create temporary issuer, an identifier issuer
			// initialized with the prefix _:b.
			issuer := NewIdentifierIssuer("_:b")

			// 6.2.3) Use the Issue Identifier algorithm, passing temporary
			// issuer and identifier, to issue a new temporary blank node
			// identifier for identifier.
			issuer.GetId(id)

			// 6.2.4) Run the Hash N-Degree Quads algorithm, passing
			// temporary issuer, and append the result to the hash path
			// list.
			hash, newIssuer := na.hashNDegreeQuads(id, issuer)
			issuerList, hasList := hashPaths[hash]
			if !hasList {
				issuerList = make([]*IdentifierIssuer, 0)
			}
			hashPaths[hash] = append(issuerList, newIssuer)
		}

		// 6.3) For each result in the hash path list,
		// lexicographically-sorted by the hash in result:
		sortedHashes := make([]string, len(hashPaths))
		i := 0
		for key := range hashPaths {
			sortedHashes[i] = key
			i++
		}
		sort.Strings(sortedHashes)
		for _, hash := range sortedHashes {
			for _, resultIssuer := range hashPaths[hash] {
				// 6.3.1) For each blank node identifier, existing identifier,
				// that was issued a temporary identifier by identifier issuer
				// in result, issue a canonical identifier, in the same order,
				// using the Issue Identifier algorithm, passing canonical
				// issuer and existing identifier.
				for _, existing := range resultIssuer.existingOrder {
					na.canonicalIssuer.GetId(existing)
				}
			}
		}
	}

	// Note: At this point all blank nodes in the set of RDF quads have been
	// assigned canonical identifiers, which have been stored in the
	// canonical issuer. Here each quad is updated by assigning each of its
	// blank nodes its new identifier.

	// 7) For each quad, quad, in input dataset:
	na.lines = make([]string, len(na.quads))
	for i, quad := range na.quads {
		// 7.1) Create a copy, quad copy, of quad and replace any existing blank
		// node identifiers using the canonical identifiers previously issued by
		// canonical issuer.
		// Note: We optimize away the copy here.
		for _, nodePtr := range []*Node{&quad.Subject, &quad.Object, &quad.Graph} {
			if *nodePtr == nil {
				continue
			}
			attrValue := (*nodePtr).GetValue()
			if IsBlankNode(*nodePtr) && !strings.HasPrefix(attrValue, "_:c14n") {
				*nodePtr = NewBlankNode(na.canonicalIssuer.GetId(attrValue))
			}
		}

		// 7.2) Add quad copy to the normalized dataset.
		var name string
		nameVal := quad.Graph
		if nameVal != nil {
			name = nameVal.GetValue()
		}
		na.lines[i] = toNQuad(quad, name)
	}

	// sort normalized output
	sort.Sort(na)
}

func (na *NormalisationAlgorithm) Main(dataset *RDFDataset, opts *JsonLdOptions) (interface{}, error) {
	// Steps 1 through 7.2, plus sorting
	na.Normalize(dataset)

	// 8) Return the normalized dataset.
	// handle output format
	if opts.Format != "" {
		if opts.Format == "application/n-quads" || opts.Format == "application/nquads" {
			rval := ""
			for _, n := range na.lines {
				rval += n
			}
			return rval, nil
		} else {
			return nil, NewJsonLdError(UnknownFormat, opts.Format)
		}
	}
	var rval []byte
	for _, n := range na.lines {
		rval = append(rval, []byte(n)...)
	}

	return ParseNQuads(string(rval))
}

// Sort interface
func (na *NormalisationAlgorithm) Len() int           { return len(na.quads) }
func (na *NormalisationAlgorithm) Less(i, j int) bool { return na.lines[i] < na.lines[j] }
func (na *NormalisationAlgorithm) Swap(i, j int) {
	na.lines[i], na.lines[j] = na.lines[j], na.lines[i]
	na.quads[i], na.quads[j] = na.quads[j], na.quads[i]
}

// 4.6) Hash First Degree Quads
func (na *NormalisationAlgorithm) hashFirstDegreeQuads(id string) string {
	// return cached hash
	info := na.blankNodeInfo[id]
	if hash, hasHash := info["hash"]; hasHash {
		return hash.(string)
	}

	// 1) Initialize nquads to an empty list. It will be used to store quads
	// in N-Quads format.
	nquads := make([]string, 0)

	// 2) Get the list of quads associated with the reference blank
	// node identifier in the blank node to quads map.
	quads := info["quads"].([]*Quad)

	// 3) For each quad quad in quads:
	for _, quad := range quads {
		// 3.1) Serialize the quad in N-Quads format with the following
		// special rule:

		// 3.1.1) If any component in quad is an blank node, then serialize
		// it using a special identifier as follows:

		// 3.1.2) If the blank node's existing blank node identifier
		// matches the reference blank node identifier then use the
		// blank node identifier _:a, otherwise, use the blank node
		// identifier _:z.
		graphCopy := na.modifyFirstDegreeComponent(id, quad.Graph, true)
		var name string
		if graphCopy != nil {
			name = graphCopy.GetValue()
		}

		quadCopy := NewQuad(
			na.modifyFirstDegreeComponent(id, quad.Subject, false),
			quad.Predicate,
			na.modifyFirstDegreeComponent(id, quad.Object, false),
			name,
		)

		nquads = append(nquads, toNQuad(quadCopy, name))
	}

	// 4) Sort nquads in lexicographical order.
	sort.Strings(nquads)

	// 5) Return the hash that results from passing the sorted, joined nquads
	// through the hash algorithm.
	hash := na.hashNQuads(nquads)
	info["hash"] = hash
	return hash
}

// helper for modifying component during Hash First Degree Quads
func (na *NormalisationAlgorithm) modifyFirstDegreeComponent(id string, component Node, isGraph bool) Node {
	if !IsBlankNode(component) {
		return component
	}
	var val string
	if na.version == AlgorithmURDNA2015 {
		if component.GetValue() == id {
			val = "_:a"
		} else {
			val = "_:z"
		}
	} else {
		if isGraph {
			val = "_:g"
		} else {
			if component.GetValue() == id {
				val = "_:a"
			} else {
				val = "_:z"
			}
		}
	}
	return NewBlankNode(val)
}

// 4.7) Hash Related Blank Node
func (na *NormalisationAlgorithm) hashRelatedBlankNode(related string, quad *Quad, issuer *IdentifierIssuer, position string) string {
	// 1) Set the identifier to use for related, preferring first the
	// canonical identifier for related if issued, second the identifier
	// issued by issuer if issued, and last, if necessary, the result of
	// the Hash First Degree Quads algorithm, passing related.
	var id string
	if na.canonicalIssuer.HasId(related) {
		id = na.canonicalIssuer.GetId(related)
	} else if issuer.HasId(related) {
		id = issuer.GetId(related)
	} else {
		id = na.hashFirstDegreeQuads(related)
	}

	// 2) Initialize a string input to the value of position.
	// Note: We use a hash object instead.
	md := na.createHash()
	md.Write([]byte(position))

	// 3) If position is not g, append <, the value of the predicate in
	// quad, and > to input.
	if position != "g" {
		md.Write([]byte(na.getRelatedPredicate(quad)))
	}

	// 4) Append identifier to input.
	md.Write([]byte(id))

	// 5) Return the hash that results from passing input through the hash
	// algorithm.
	return encodeHex(md.Sum(nil))
}

// 4.8) Hash N-Degree Quads
func (na *NormalisationAlgorithm) hashNDegreeQuads(id string, issuer *IdentifierIssuer) (string, *IdentifierIssuer) {
	// 1) Create a hash to related blank nodes map for storing hashes that
	// identify related blank nodes.
	// Note: 2) and 3) handled within `createHashToRelated`
	hashToRelated := na.createHashToRelated(id, issuer)

	// 4) Create an empty string, data to hash.
	// Note: We create a hash object instead.
	md := na.createHash()

	// 5) For each related hash to blank node list mapping in hash to
	// related blank nodes map, sorted lexicographically by related hash:
	sortedHashes := make([]string, len(hashToRelated))
	i := 0
	for key := range hashToRelated {
		sortedHashes[i] = key
		i++
	}
	sort.Strings(sortedHashes)
	for _, hash := range sortedHashes {
		blankNodes := hashToRelated[hash]
		// 5.1) Append the related hash to the data to hash.
		md.Write([]byte(hash))

		// 5.2) Create a string chosen path.
		chosenPath := ""

		// 5.3) Create an unset chosen issuer variable.
		var chosenIssuer *IdentifierIssuer

		// 5.4) For each permutation of blank node list:
		permutator := NewPermutator(blankNodes)
		for permutator.HasNext() {
			permutation := permutator.Next()

			// 5.4.1) Create a copy of issuer, issuer copy.
			issuerCopy := issuer.Clone()

			// 5.4.2) Create a string path.
			path := ""

			// 5.4.3) Create a recursion list, to store blank node
			// identifiers that must be recursively processed by this
			// algorithm.
			recursionList := make([]string, 0)

			// 5.4.4) For each related in permutation:
			skipToNextPermutation := false

			for _, related := range permutation {
				// 5.4.4.1) If a canonical identifier has been issued for
				// related, append it to path.
				if na.canonicalIssuer.HasId(related) {
					path += na.canonicalIssuer.GetId(related)
				} else {
					// 5.4.4.2) Otherwise:

					// 5.4.4.2.1) If issuer copy has not issued an
					// identifier for related, append related to recursion
					// list.
					if !issuerCopy.HasId(related) {
						recursionList = append(recursionList, related)
					}

					// 5.4.4.2.2) Use the Issue Identifier algorithm,
					// passing issuer copy and related and append the result
					// to path.
					path += issuerCopy.GetId(related)
				}
				// 5.4.4.3) If chosen path is not empty and the length of
				// path is greater than or equal to the length of chosen
				// path and path is lexicographically greater than chosen
				// path, then skip to the next permutation.
				if len(chosenPath) != 0 && len(path) >= len(chosenPath) && path > chosenPath {
					skipToNextPermutation = true
					break
				}
			}

			if skipToNextPermutation {
				continue
			}

			// 5.4.5) For each related in recursion list:
			for _, related := range recursionList {
				// 5.4.5.1) Set result to the result of recursively
				// executing the Hash N-Degree Quads algorithm, passing
				// related for identifier and issuer copy for path
				// identifier issuer.
				resultHash, resultIssuer := na.hashNDegreeQuads(related, issuerCopy)

				// 5.4.5.2) Use the Issue Identifier algorithm, passing
				// issuer copy and related and append the result to path.
				path += issuerCopy.GetId(related)

				// 5.4.5.3) Append <, the hash in result, and > to path.
				path += "<" + resultHash + ">"

				// 5.4.5.4) Set issuer copy to the identifier issuer in
				// result.
				issuerCopy = resultIssuer

				// 5.4.5.5) If chosen path is not empty and the length of
				// path is greater than or equal to the length of chosen
				// path and path is lexicographically greater than chosen
				// path, then skip to the next permutation.
				if len(chosenPath) != 0 && len(path) >= len(chosenPath) && path > chosenPath {
					skipToNextPermutation = true
					break
				}
			}

			if skipToNextPermutation {
				continue
			}

			// 5.4.6) If chosen path is empty or path is lexicographically
			// less than chosen path, set chosen path to path and chosen
			// issuer to issuer copy.
			if len(chosenPath) == 0 || path < chosenPath {
				chosenPath = path
				chosenIssuer = issuerCopy
			}
		}

		// 5.5) Append chosen path to data to hash.
		md.Write([]byte(chosenPath))

		// 5.6) Replace issuer, by reference, with chosen issuer.
		issuer = chosenIssuer
	}
	// 6) Return issuer and the hash that results from passing data to hash
	// through the hash algorithm.
	return encodeHex(md.Sum(nil)), issuer
}

// helper to create appropriate hash object
func (na *NormalisationAlgorithm) createHash() hashPkg.Hash {
	if na.version == AlgorithmURDNA2015 {
		return sha256.New()
	} else {
		return sha1.New() //nolint:gosec
	}
}

// helper to hash a list of nquads
func (na *NormalisationAlgorithm) hashNQuads(nquads []string) string {
	h := na.createHash()
	for _, nquad := range nquads {
		h.Write([]byte(nquad))
	}
	return encodeHex(h.Sum(nil))
}

// helper for getting a related predicate
func (na *NormalisationAlgorithm) getRelatedPredicate(quad *Quad) string {
	if na.version == AlgorithmURDNA2015 {
		return "<" + quad.Predicate.GetValue() + ">"
	} else {
		return quad.Predicate.GetValue()
	}
}

// helper for creating hash to related blank nodes map
func (na *NormalisationAlgorithm) createHashToRelated(id string, issuer *IdentifierIssuer) map[string][]string {
	// 1) Create a hash to related blank nodes map for storing hashes that
	// identify related blank nodes.
	hashToRelated := make(map[string][]string)

	// 2) Get a reference, quads, to the list of quads in the blank node to
	// quads map for the key identifier.
	quads := na.blankNodeInfo[id]["quads"].([]*Quad)

	// 3) For each quad in quads:
	var related, position string
	if na.version == AlgorithmURDNA2015 {
		for _, quad := range quads {
			// 3.1) For each component in quad, if component is the subject,
			// object, and graph name and it is a blank node that is not
			// identified by identifier:
			i := 0
			for _, attrNode := range []Node{quad.Subject, quad.Object, quad.Graph} {
				if attrNode != nil {
					attrValue := attrNode.GetValue()
					if IsBlankNode(attrNode) && attrValue != id {
						// 3.1.1) Set hash to the result of the Hash Related Blank
						// Node algorithm, passing the blank node identifier for
						// component as related, quad, path identifier issuer as
						// issuer, and position as either s, o, or g based on
						// whether component is a subject, object, graph name,
						// respectively.
						related = attrValue
						position = Positions[i]
						hash := na.hashRelatedBlankNode(related, quad, issuer, position)

						// 3.1.2) Add a mapping of hash to the blank node identifier
						// for component to hash to related blank nodes map, adding
						// an entry as necessary.
						relatedList, hasHash := hashToRelated[hash]
						if !hasHash {
							relatedList = make([]string, 0)
						}
						hashToRelated[hash] = append(relatedList, related)
					}
				}
				i++
			}
		}
	} else {
		for _, quad := range quads {
			// 3.1) If the quad's subject is a blank node that does not match
			// identifier, set hash to the result of the Hash Related Blank Node
			// algorithm, passing the blank node identifier for subject as
			// related, quad, path identifier issuer as issuer, and p as
			// position.
			if IsBlankNode(quad.Subject) && quad.Subject.GetValue() != id {
				related = quad.Subject.GetValue()
				position = "p"
			} else if IsBlankNode(quad.Object) && quad.Object.GetValue() != id {
				// 3.2) Otherwise, if quad's object is a blank node that does
				// not match identifier, to the result of the Hash Related Blank
				// Node algorithm, passing the blank node identifier for object
				// as related, quad, path identifier issuer as issuer, and r
				// as position.
				related = quad.Object.GetValue()
				position = "r"
			} else {
				continue
			}

			// 3.4) Add a mapping of hash to the blank node identifier for the
			// component that matched (subject or object) to hash to related
			// blank nodes map, adding an entry as necessary.
			hash := na.hashRelatedBlankNode(related, quad, issuer, position)
			relatedList, hasHash := hashToRelated[hash]
			if !hasHash {
				relatedList = make([]string, 0)
			}
			hashToRelated[hash] = append(relatedList, related)
		}
	}
	return hashToRelated
}

const hexDigit = "0123456789abcdef"

func encodeHex(data []byte) string {
	var buf = make([]byte, 0, len(data)*2)
	for _, b := range data {
		buf = append(buf, hexDigit[b>>4], hexDigit[b&0xf])
	}
	return string(buf)
}

// Permutator
type Permutator struct {
	list []string
	done bool
	left map[string]bool
}

// NewPermutator creates a new instance of Permutator.
func NewPermutator(list []string) *Permutator {
	p := &Permutator{}
	p.list = make([]string, len(list))
	copy(p.list, list)
	sort.Strings(p.list)
	p.done = false
	p.left = make(map[string]bool, len(list))
	for _, i := range p.list {
		p.left[i] = true
	}

	return p
}

// HasNext returns true if there is another permutation.
func (p *Permutator) HasNext() bool {
	return !p.done
}

// Next gets the next permutation. Call HasNext() to ensure there is another one first.
func (p *Permutator) Next() []string {
	rval := make([]string, len(p.list))
	copy(rval, p.list)

	// Calculate the next permutation using Steinhaus-Johnson-Trotter
	// permutation algorithm

	// get largest mobile element k
	// (mobile: element is greater than the one it is looking at)
	k := ""
	pos := 0
	length := len(p.list)
	for i := 0; i < length; i++ {
		element := p.list[i]
		left := p.left[element]
		if (k == "" || element > k) &&
			((left && i > 0 && element > p.list[i-1]) || (!left && i < (length-1) && element > p.list[i+1])) {
			k = element
			pos = i
		}
	}

	// no more permutations
	if k == "" {
		p.done = true
	} else {
		// swap k and the element it is looking at
		var swap int
		if p.left[k] {
			swap = pos - 1
		} else {
			swap = pos + 1
		}
		p.list[pos] = p.list[swap]
		p.list[swap] = k

		// reverse the direction of all element larger than k
		for i := 0; i < length; i++ {
			if p.list[i] > k {
				p.left[p.list[i]] = !p.left[p.list[i]]
			}
		}
	}

	return rval
}
