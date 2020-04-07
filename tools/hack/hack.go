package main

import (
	"encoding/json"
	"fmt"
	"os"

	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/ghodss/yaml"
)

const filename = "./manifests/generated/vm-resource.yaml"

func main() {
	file, _ := os.OpenFile(filename, os.O_RDWR, 0644)
	crd := extv1beta1.CustomResourceDefinition{}
	err := k8syaml.NewYAMLToJSONDecoder(file).Decode(&crd)
	if err != nil {
		panic(fmt.Errorf("Failed to parse crd from file %v, %v", filename, err))
	}

	metadata := crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties["dataVolumeTemplates"].Items.Schema.Properties["metadata"]
	metadata.Nullable = true
	t := true
	metadata.XPreserveUnknownFields = &t
	crd.Spec.Validation.OpenAPIV3Schema.Properties["spec"].Properties["dataVolumeTemplates"].Items.Schema.Properties["metadata"] = metadata

	file.Close()
	os.Remove(filename)

	file, _ = os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	defer file.close()
	jsonBytes, err := json.Marshal(crd)
	if err != nil {
		panic(err)
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		panic(err)
	}

	// remove status
	unstructured.RemoveNestedField(r.Object, "status")

	b, _ := yaml.Marshal(r.Object)
	file.Write(b)
}
