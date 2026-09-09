// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

package cel

import (
	"testing"

	"github.com/run-ai/karta/pkg/api/runai/v1alpha1"
)

func TestZZVerifyKeywordReferenceNames(t *testing.T) {
	keywords := []string{"in", "true", "false", "null"}
	reserved := []string{"namespace", "loop", "package", "as", "if", "while"}

	for _, name := range append(append([]string{}, keywords...), reserved...) {
		karta := &v1alpha1.Karta{
			Spec: v1alpha1.KartaSpec{
				StructureDefinition: v1alpha1.StructureDefinition{
					RootComponent: v1alpha1.ComponentDefinition{
						Name: "root",
						GVK:  v1alpha1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
					},
					References: []v1alpha1.ResourceReference{{
						Name:   name,
						GVK:    v1alpha1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
						Lookup: &v1alpha1.LookupReference{NameExpression: `object.metadata.name`},
					}},
				},
			},
		}
		err := v1alpha1.NewKartaValidator(karta).Validate()
		t.Logf("Validate() with reference name %q: err=%v", name, err)
	}

	for _, name := range append(append([]string{}, keywords...), reserved...) {
		err := Validate("references." + name)
		t.Logf("compile %-25q err=%v", "references."+name, err)
	}

	// Index-syntax workaround
	for _, name := range keywords {
		err := Validate(`references["` + name + `"]`)
		t.Logf("compile %-25q err=%v", `references["`+name+`"]`, err)
	}
}
