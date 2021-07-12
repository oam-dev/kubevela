/*
 Copyright 2021. The KubeVela Authors.

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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/oam-dev/kubevela/apis/core.oam.dev/v1beta1"
)

const (
	// InitializerTemplateName represents the Initializer template file of addons
	InitializerTemplateName = "template.yaml"

	// InitializerFileDir is where we store generated initializer & component definition
	InitializerFileDir = "demo"

	// ComponentDefDir is where we store correspond componentDefinition for addon
	ComponentDefDir = "definitions"

	// ResourceDir is where we store correspond componentDefinition for addon
	ResourceDir = "resource"

	// DescAnnotation records the description of addon
	DescAnnotation = "addons.oam.dev/description"

	// MarkLabel is annotation key marks configMap as an addon
	MarkLabel = "addons.oam.dev/type"
)

type velaFile struct {
	RelativePath string
	Name         string
	Content      string
}

// AddonInfo records addon's metadata
type AddonInfo struct {
	ResourceFiles   []velaFile
	DefinitionFiles []velaFile
	Name            string
	Namespace       string
	Description     string
	TemplatePath    string
}

func walkAllAddons(path string) ([]string, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	addons := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() && file.Name() != InitializerFileDir {
			addons = append(addons, file.Name())
		}
	}
	return addons, nil
}

func indentedContent(content string, indent int) string {
	var res string
	lines := strings.Split(content, "\n")
	indentSpace := strings.Repeat(" ", indent)
	for i, line := range lines {
		res += indentSpace + line
		if i != len(lines)-1 {
			res += "\n"
		}
	}
	return res
}

func getAddonInfo(addon string, addonsPath string) (*AddonInfo, error) {
	addonRoot := filepath.Clean(addonsPath + "/" + addon)
	resourceRoot := filepath.Clean(addonRoot + "/" + ResourceDir)
	defRoot := filepath.Clean(addonRoot + "/" + ComponentDefDir)
	resourcesFiles := make([]velaFile, 0, 2)
	defFiles := make([]velaFile, 0, 2)
	addInfo := &AddonInfo{
		Name:         addon,
		TemplatePath: filepath.Join(addonRoot, InitializerTemplateName),
	}
	if err := filepath.Walk(resourceRoot, func(path string, info fs.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		content, err := ioutil.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		obj := new(unstructured.Unstructured)
		if err = yaml.Unmarshal(content, obj); err != nil {
			return err
		}
		resourcesFiles = append(resourcesFiles, velaFile{
			RelativePath: path,
			Name:         obj.GetName(),
			Content:      indentedContent(string(content), 10),
		})
		return nil
	}); err != nil {
		return nil, err
	}

	addInfo.ResourceFiles = resourcesFiles
	if err := filepath.Walk(defRoot, func(path string, info fs.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}
		content, err := ioutil.ReadFile(filepath.Clean(path))
		if err != nil {
			return err
		}

		obj := new(unstructured.Unstructured)
		if err = yaml.Unmarshal(content, obj); err != nil {
			return err
		}
		defFiles = append(defFiles, velaFile{
			RelativePath: path,
			Name:         obj.GetName(),
			Content:      string(content),
		})
		return nil
	}); err != nil {
		return nil, err
	}
	addInfo.DefinitionFiles = defFiles
	return addInfo, nil
}

// WriteToFile write files
func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		if err = file.Close(); err != nil {
			fmt.Printf("Error closing file: %s\n", err)
		}
	}()

	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

func generateInitializer(addon *AddonInfo) (*v1beta1.Initializer, error) {
	t, err := template.ParseFiles(addon.TemplatePath)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, addon)
	if err != nil {
		return nil, err
	}

	init := new(v1beta1.Initializer)
	err = yaml.Unmarshal(buf.Bytes(), init)
	if err != nil {
		return nil, err
	}
	return init, err
}

func setConfigMapLabels(addonInfo *AddonInfo) map[string]string {
	return map[string]string{
		MarkLabel: addonInfo.Name,
	}
}
func setConfigMapAnnotations(addonInfo *AddonInfo) map[string]string {
	return map[string]string{
		DescAnnotation: addonInfo.Description,
	}
}
func removeTimestampInplace(s *string) {
	clearStr := "(\n.*?metadata:.*?)?\n.*?creationTimestamp:.*?null"
	var re = regexp.MustCompile(clearStr)
	*s = re.ReplaceAllString(*s, "")
}

func storeConfigMap(addonInfo *AddonInfo, initializer *v1beta1.Initializer, storePath string) error {
	configMap := &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
	}
	addonInfo.Description = initializer.GetAnnotations()[DescAnnotation]
	configMap.SetName(addonInfo.Name)
	configMap.SetNamespace(addonInfo.Namespace)
	configMap.SetAnnotations(setConfigMapAnnotations(addonInfo))
	configMap.SetLabels(setConfigMapLabels(addonInfo))

	data := make(map[string]string, 1)
	initContent, err := yaml.Marshal(initializer)
	if err != nil {
		return err
	}
	data["initializer"] = string(initContent)
	configMap.Data = data
	content, err := yaml.Marshal(configMap)
	if err != nil {
		return err
	}
	raw := string(content)
	removeTimestampInplace(&raw)
	filename := storePath + "/" + addonInfo.Name + ".yaml"
	return WriteToFile(filename, raw)
}

func storeInitAndDef(init *v1beta1.Initializer, cds []*v1beta1.ComponentDefinition, addonPath string, addonName string) error {
	initContent, err := yaml.Marshal(init)
	if err != nil {
		return err
	}
	filename := path.Join(addonPath, InitializerFileDir, addonName+".yaml")
	spliter := "---\n"
	cdContents := make([]string, 0, len(cds))
	for _, cd := range cds {
		cdContent, err := yaml.Marshal(cd)
		if err != nil {
			return err
		}
		cdContents = append(cdContents, string(cdContent))
	}
	fileContent := strings.Join(append(cdContents, string(initContent)), spliter)
	return WriteToFile(filename, fileContent)
}

func getComponentDefs(info *AddonInfo) ([]*v1beta1.ComponentDefinition, error) {
	cds := make([]*v1beta1.ComponentDefinition, 0)
	for _, file := range info.DefinitionFiles {
		cd := v1beta1.ComponentDefinition{}
		err := yaml.Unmarshal([]byte(file.Content), &cd)
		if err != nil {
			return nil, err
		}
		cds = append(cds, &cd)
	}
	return cds, nil

}
func main() {
	var addonsPath string
	var storePath string

	flag.StringVar(&addonsPath, "addons-path", "", "addons path")
	flag.StringVar(&storePath, "store-path", "", "path store configMap")
	flag.Parse()

	addons, err := walkAllAddons(addonsPath)
	dealErr := func(err error) {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, addon := range addons {
		addInfo, err := getAddonInfo(addon, addonsPath)
		dealErr(err)
		init, err := generateInitializer(addInfo)
		dealErr(err)
		cds, err := getComponentDefs(addInfo)
		dealErr(err)
		err = storeInitAndDef(init, cds, addonsPath, addInfo.Name)
		dealErr(err)
		err = storeConfigMap(addInfo, init, storePath)
		dealErr(err)
	}
}
