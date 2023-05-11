package usb

import (
	"fmt"
	"os"
	"strings"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/util"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

func CreateHostDevices(vmiHostDevices []v1.HostDevice) ([]api.HostDevice, error) {
	hostdevices := []api.HostDevice{}
	for _, device := range vmiHostDevices {
		env := util.ResourceNameToEnvVar("USB", device.DeviceName)
		addressString, ok := os.LookupEnv(env)
		if !ok {
			return nil, fmt.Errorf("todo")
		}
		evnS := strings.Split(addressString, ":")
		bus, device := evnS[0], evnS[1]
		hostdevices = append(hostdevices,
			api.HostDevice{
				Type:  "usb",
				Mode:  "subsystem",
				Alias: api.NewUserDefinedAlias("usb-host"),
				Source: api.HostDeviceSource{
					Address: &api.Address{
						Bus:    bus,
						Device: device,
					},
				},
			})
	}
	return hostdevices, nil
}
