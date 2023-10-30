package libvmi

import v1 "kubevirt.io/api/core/v1"

func Apply(vmi *v1.VirtualMachineInstance, options ...Option) {
	for _, option := range options {
		option(vmi)
	}
}
