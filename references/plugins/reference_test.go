/*
Copyright 2021 The KubeVela Authors.

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

package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/oam-dev/kubevela/apis/types"
)

var RefTestDir = filepath.Join(TestDir, "ref")

func TestCreateRefTestDir(t *testing.T) {
	if _, err := os.Stat(RefTestDir); err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(RefTestDir, 0750)
		assert.NoError(t, err)
	}
}

func TestCreateMarkdown(t *testing.T) {
	workloadName := "workload1"
	traitName := "trait1"
	scopeName := "scope1"

	workloadCueTemplate := `
parameter: {
	// +usage=Which image would you like to use for your service
	// +short=i
	image: string
}
`
	traitCueTemplate := `
parameter: {
	replicas: int
}
`

	cases := map[string]struct {
		reason       string
		capabilities []types.Capability
		want         error
	}{
		"WorkloadTypeAndTraitCapability": {
			reason: "valid capabilities",
			capabilities: []types.Capability{
				{
					Name:        workloadName,
					Type:        types.TypeWorkload,
					CueTemplate: workloadCueTemplate,
				},
				{
					Name:        traitName,
					Type:        types.TypeTrait,
					CueTemplate: traitCueTemplate,
				},
			},
			want: nil,
		},
		"ScopeTypeCapability": {
			reason: "invalid capabilities",
			capabilities: []types.Capability{
				{
					Name: scopeName,
					Type: types.TypeScope,
				},
			},
			want: fmt.Errorf("the type of the capability is not right"),
		},
	}
	ref := &MarkdownReference{}
	ctx := context.Background()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ref.CreateMarkdown(ctx, tc.capabilities, RefTestDir, ReferenceSourcePath)
			if diff := cmp.Diff(tc.want, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCreateMakrdown(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}

}

func TestPrepareParameterTable(t *testing.T) {
	ref := MarkdownReference{}
	tableName := "hello"
	var depth int = 1
	parameterList := []ReferenceParameter{
		{
			PrintableType: "string",
			Depth:         &depth,
		},
	}
	parameterName := "cpu"
	parameterList[0].Name = parameterName
	parameterList[0].Required = true
	refContent := ref.prepareParameter(tableName, parameterList, types.CUECategory)
	assert.Contains(t, refContent, parameterName)
	assert.Contains(t, refContent, "cpu")
}

func TestDeleteRefTestDir(t *testing.T) {
	if _, err := os.Stat(RefTestDir); err == nil {
		err := os.RemoveAll(RefTestDir)
		assert.NoError(t, err)
	}
}
