package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubevirt.io/api/usb"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion     = schema.GroupVersion{Group: usb.GroupName, Version: usb.Version}
	SchemeGroupVersionKind = schema.GroupVersionKind{Group: usb.GroupName, Version: usb.Version, Kind: usb.Kind}

	ResourceNodeConfigSingular = "nodeconfig"
	ResourceNodeConfigPlural   = ResourceNodeConfigSingular + "s"
)

var (
	// SchemeBuilder initializes a scheme builder
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme is a global function that registers this API group & version to a scheme
	AddToScheme = SchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&NodeConfig{},
		&NodeConfigList{})

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
