package virthandler

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1 "kubevirt.io/client-go/api/v1"
	diskutils "kubevirt.io/kubevirt/pkg/ephemeral-disk-utils"
	hostdisk "kubevirt.io/kubevirt/pkg/host-disk"
	"kubevirt.io/kubevirt/pkg/virt-handler/isolation"
)

func changeOwnershipOfHostDisks(vmiWithAllPVCs *v1.VirtualMachineInstance, res isolation.IsolationResult) error {
	for i := range vmiWithAllPVCs.Spec.Volumes {
		if volumeSource := &vmiWithAllPVCs.Spec.Volumes[i].VolumeSource; volumeSource.HostDisk != nil {
			volumeName := vmiWithAllPVCs.Spec.Volumes[i].Name
			diskPath := hostdisk.GetMountedHostDiskPath(volumeName, volumeSource.HostDisk.Path)
			err := diskutils.DefaultOwnershipManager.SetFileOwnership(filepath.Join(res.MountRoot(), diskPath))
			switch err.(type) {
			case *os.PathError:
				diskDir := hostdisk.GetMountedHostDiskDir(volumeName)
				if err := diskutils.DefaultOwnershipManager.SetFileOwnership(filepath.Join(res.MountRoot(), diskDir)); err != nil {
					return fmt.Errorf("Failed to change ownership of HostDisk dir %s, %s", volumeName, err)
				}

				// get the right context here
				unprivilegedContainerSELinuxLabel := "system_u:object_r:container_file_t:s0"
				err = relabelFiles(unprivilegedContainerSELinuxLabel, filepath.Join(res.MountRoot(), diskDir))
				if err != nil {
					panic(fmt.Errorf("error relabeling required files: %v", err))
				}

			case nil:
				unprivilegedContainerSELinuxLabel := "system_u:object_r:container_file_t:s0"
				err = relabelFiles(unprivilegedContainerSELinuxLabel, filepath.Join(res.MountRoot(), diskPath))
				if err != nil {
					panic(fmt.Errorf("error relabeling required files: %v", err))
				}
			default:
				return fmt.Errorf("Failed to change ownership of HostDisk %s, %s", volumeName, err)
			}

		}
	}
	return nil
}

func changeOwnershipOfBlockDevices(vmiWithOnlyBlockPVCs *v1.VirtualMachineInstance, res isolation.IsolationResult) error {
	for i := range vmiWithOnlyBlockPVCs.Spec.Volumes {
		if volumeSource := &vmiWithOnlyBlockPVCs.Spec.Volumes[i].VolumeSource; volumeSource.PersistentVolumeClaim != nil {
			devPath := filepath.Join(string(filepath.Separator), "dev", vmiWithOnlyBlockPVCs.Spec.Volumes[i].Name)
			if err := diskutils.DefaultOwnershipManager.SetFileOwnership(filepath.Join(res.MountRoot(), devPath)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *VirtualMachineController) prepareStorage(vmiWithOnlyBlockPVCS, vmiWithAllPVCs *v1.VirtualMachineInstance) error {
	res, err := d.podIsolationDetector.Detect(vmiWithOnlyBlockPVCS)
	if err != nil {
		return err
	}
	if err := changeOwnershipOfHostDisks(vmiWithAllPVCs, res); err != nil {
		return err
	}

	return changeOwnershipOfBlockDevices(vmiWithOnlyBlockPVCS, res)
}

func (d *VirtualMachineController) prepareTap(vmi *v1.VirtualMachineInstance) error {
	macvtap := map[string]bool{}
	for _, inf := range vmi.Spec.Domain.Devices.Interfaces {
		if inf.Macvtap != nil {
			macvtap[inf.Name] = true
		}
	}

	netDevices := []string{}
	for _, net := range vmi.Spec.Networks {
		_, ok := macvtap[net.Name]
		if ok {
			netDevices = append(netDevices, net.Multus.NetworkName)
		}
	}
	res, err := d.podIsolationDetector.Detect(vmi)
	if err != nil {
		return err
	}

	for _, tap := range netDevices {
		path := filepath.Join(res.MountRoot(), "sys", "class", "net", tap, "ifindex")
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read if index, %v", err)
		}

		index, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return err
		}

		pathToTap := filepath.Join(res.MountRoot(), "dev", fmt.Sprintf("tap%d", index))

		if err := diskutils.DefaultOwnershipManager.SetFileOwnership(pathToTap); err != nil {
			return err
		}
	}

	return nil

}

func (d *VirtualMachineController) prepareHugePages(vmi *v1.VirtualMachineInstance) error {
	if vmi.Spec.Domain.Memory == nil || vmi.Spec.Domain.Memory.Hugepages == nil {
		return nil
	}

	res, err := d.podIsolationDetector.Detect(vmi)
	if err != nil {
		return err
	}
	hugepages := filepath.Join(res.MountRoot(), "dev", "hugepages")
	if err := diskutils.DefaultOwnershipManager.SetFileOwnership(hugepages); err != nil {
		return err
	}
	return nil
}

func (d *VirtualMachineController) nonRootSetUp(origVMI, vmi *v1.VirtualMachineInstance) error {
	if err := d.prepareStorage(vmi, origVMI); err != nil {
		return err
	}
	if err := d.prepareTap(origVMI); err != nil {
		return err
	}

	if err := d.prepareHugePages(origVMI); err != nil {
		return err
	}
	return nil
}
