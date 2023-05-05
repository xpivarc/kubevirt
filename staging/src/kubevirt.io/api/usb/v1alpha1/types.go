package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeConfig represents a subset of Node resources that we want to expose
// to virtual machines.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NodeConfigSpec   `json:"spec" valid:"required"`
	Status            NodeConfigStatus `json:"status,omitempty"`
}

type NodeConfigSpec struct {
	USB USB `json:"usb,omitempty"`
}

type USB struct {
	// TODO
	ResourceName string `json:"resourceName"`
	// +listType=atomic
	USBHostDevices []USBHostDevices `json:"usbHostDevices,omitempty"`
}

type USBHostDevices struct {
	// This is the selector for the USB device. To identify a USB device in a node, the minimum
	// necessary is its vendor:product information. As we could have multiple devices with the
	// same vendor:product information, a few other optional informations can be used to select the
	// device, by using either serial number or specifying device's Bus and Device Number:
	// Minimal example: "0951:1666"
	// TODO:
	// With serial number: "0951:1666,serial=E0D55E6CBD23E691583A004D"
	// With bus and device number: "0951:1666,bus=2,device=4"
	USBVendorSelector string `json:"usbVendorSelector"`
}

// TODO
type NodeConfigStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// +listType=atomic
	Items []NodeConfig `json:"items"`
}
