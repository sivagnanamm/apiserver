/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/schemaconv"
	"k8s.io/kube-openapi/pkg/util/proto"
	smdschema "sigs.k8s.io/structured-merge-diff/schema"
	"sigs.k8s.io/structured-merge-diff/typed"
)

// groupVersionKindExtensionKey is the key used to lookup the
// GroupVersionKind value for an object definition from the
// definition's "extensions" map.
const groupVersionKindExtensionKey = "x-kubernetes-group-version-kind"

type gvkParser struct {
	gvks   map[schema.GroupVersionKind]string
	parser typed.Parser
}

func (p *gvkParser) Type(gvk schema.GroupVersionKind) typed.ParseableType {
	typeName, ok := p.gvks[gvk]
	if !ok {
		return nil
	}
	return p.parser.Type(typeName)
}

func newGVKParser(models proto.Models) (*gvkParser, error) {
	typeSchema, err := schemaconv.ToSchema(models)
	if err != nil {
		return nil, fmt.Errorf("failed to convert models to schema: %v", err)
	}
	typeSchema = makeRawExtensionUntyped(typeSchema)
	parser := gvkParser{
		gvks: map[schema.GroupVersionKind]string{},
	}
	parser.parser = typed.Parser{Schema: *typeSchema}
	for _, modelName := range models.ListModels() {
		model := models.LookupModel(modelName)
		if model == nil {
			panic("ListModels returns a model that can't be looked-up.")
		}
		gvkList := parseGroupVersionKind(model)
		for _, gvk := range gvkList {
			if len(gvk.Kind) > 0 {
				parser.gvks[gvk] = modelName
			}
		}
	}
	return &parser, nil
}

// Get and parse GroupVersionKind from the extension. Returns empty if it doesn't have one.
func parseGroupVersionKind(s proto.Schema) []schema.GroupVersionKind {
	extensions := s.GetExtensions()

	gvkListResult := []schema.GroupVersionKind{}

	// Get the extensions
	gvkExtension, ok := extensions[groupVersionKindExtensionKey]
	if !ok {
		return []schema.GroupVersionKind{}
	}

	// gvk extension must be a list of at least 1 element.
	gvkList, ok := gvkExtension.([]interface{})
	if !ok {
		return []schema.GroupVersionKind{}
	}

	for _, gvk := range gvkList {
		// gvk extension list must be a map with group, version, and
		// kind fields
		gvkMap, ok := gvk.(map[interface{}]interface{})
		if !ok {
			continue
		}
		group, ok := gvkMap["group"].(string)
		if !ok {
			continue
		}
		version, ok := gvkMap["version"].(string)
		if !ok {
			continue
		}
		kind, ok := gvkMap["kind"].(string)
		if !ok {
			continue
		}

		gvkListResult = append(gvkListResult, schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		})
	}

	return gvkListResult
}

// makeRawExtensionUntyped explicitly sets RawExtension's type in the schema to Untyped atomic
// TODO: remove this once kube-openapi is updated to include
// https://github.com/kubernetes/kube-openapi/pull/133
func makeRawExtensionUntyped(s *smdschema.Schema) *smdschema.Schema {
	s2 := &smdschema.Schema{}
	for _, t := range s.Types {
		t2 := t
		if t2.Name == "io.k8s.apimachinery.pkg.runtime.RawExtension" {
			t2.Atom = smdschema.Atom{
				Untyped: &smdschema.Untyped{},
			}
		}
		s2.Types = append(s2.Types, t2)
	}
	return s2
}
