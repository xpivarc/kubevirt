package cmdclientmanager

import (
	v1 "kubevirt.io/api/core/v1"
	cmdclient "kubevirt.io/kubevirt/pkg/virt-handler/cmd-client"
)

type ClientManager interface {
	CreateClient(vmi *v1.VirtualMachineInstance) (cmdclient.LauncherClient, error)
	Proxy(vmi *v1.VirtualMachineInstance) error
}
