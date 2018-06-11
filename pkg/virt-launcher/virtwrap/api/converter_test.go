/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2017, 2018 Red Hat, Inc.
 *
 */

package api

import (
	"encoding/xml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	k8sv1 "k8s.io/api/core/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
	"os"

	"kubevirt.io/kubevirt/pkg/api/v1"
)

var _ = Describe("Converter", func() {

	Context("with v1.Disk", func() {
		It("Should add boot order when provided", func() {
			order := uint(1)
			kubevirtDisk := &v1.Disk{
				Name:       "mydisk",
				BootOrder:  &order,
				VolumeName: "myvolume",
				DiskDevice: v1.DiskDevice{
					Disk: &v1.DiskTarget{
						Bus: "virtio",
					},
				},
			}
			var convertedDisk = `<Disk device="disk" type="">
  <source></source>
  <target bus="virtio" dev="vda"></target>
  <driver name="qemu" type=""></driver>
  <alias name="mydisk"></alias>
  <boot order="1"></boot>
</Disk>`
			xml := diskToDiskXML(kubevirtDisk)
			fmt.Println(xml)
			Expect(xml).To(Equal(convertedDisk))
		})

		It("Should omit boot order when not provided", func() {
			kubevirtDisk := &v1.Disk{
				Name:       "mydisk",
				VolumeName: "myvolume",
				DiskDevice: v1.DiskDevice{
					Disk: &v1.DiskTarget{
						Bus: "virtio",
					},
				},
			}
			var convertedDisk = `<Disk device="disk" type="">
  <source></source>
  <target bus="virtio" dev="vda"></target>
  <driver name="qemu" type=""></driver>
  <alias name="mydisk"></alias>
</Disk>`
			xml := diskToDiskXML(kubevirtDisk)
			fmt.Println(xml)
			Expect(xml).To(Equal(convertedDisk))
		})

	})

	Context("with v1.VirtualMachineInstance", func() {

		var vmi *v1.VirtualMachineInstance
		_false := false
		_true := true
		domainType := "kvm"
		if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
			domainType = "qemu"
		}

		BeforeEach(func() {

			vmi = &v1.VirtualMachineInstance{
				ObjectMeta: k8smeta.ObjectMeta{
					Name:      "testvmi",
					Namespace: "mynamespace",
				},
			}
			v1.SetObjectDefaults_VirtualMachineInstance(vmi)
			vmi.Spec.Domain.Devices.Watchdog = &v1.Watchdog{
				Name: "mywatchdog",
				WatchdogDevice: v1.WatchdogDevice{
					I6300ESB: &v1.I6300ESBWatchdog{
						Action: v1.WatchdogActionPoweroff,
					},
				},
			}
			vmi.Spec.Domain.Clock = &v1.Clock{
				ClockOffset: v1.ClockOffset{
					UTC: &v1.ClockOffsetUTC{},
				},
				Timer: &v1.Timer{
					HPET: &v1.HPETTimer{
						Enabled:    &_false,
						TickPolicy: v1.HPETTickPolicyDelay,
					},
					KVM: &v1.KVMTimer{
						Enabled: &_true,
					},
					PIT: &v1.PITTimer{
						Enabled:    &_false,
						TickPolicy: v1.PITTickPolicyDiscard,
					},
					RTC: &v1.RTCTimer{
						Enabled:    &_true,
						TickPolicy: v1.RTCTickPolicyCatchup,
						Track:      v1.TrackGuest,
					},
					Hyperv: &v1.HypervTimer{
						Enabled: &_true,
					},
				},
			}
			vmi.Spec.Domain.Features = &v1.Features{
				APIC: &v1.FeatureAPIC{},
				Hyperv: &v1.FeatureHyperv{
					Relaxed:    &v1.FeatureState{Enabled: &_false},
					VAPIC:      &v1.FeatureState{Enabled: &_true},
					Spinlocks:  &v1.FeatureSpinlocks{Enabled: &_true},
					VPIndex:    &v1.FeatureState{Enabled: &_true},
					Runtime:    &v1.FeatureState{Enabled: &_false},
					SyNIC:      &v1.FeatureState{Enabled: &_true},
					SyNICTimer: &v1.FeatureState{Enabled: &_false},
					Reset:      &v1.FeatureState{Enabled: &_true},
					VendorID:   &v1.FeatureVendorID{Enabled: &_false, VendorID: "myvendor"},
				},
			}
			vmi.Spec.Domain.Devices.Disks = []v1.Disk{
				{
					Name:       "mydisk",
					VolumeName: "myvolume",
					DiskDevice: v1.DiskDevice{
						Disk: &v1.DiskTarget{
							Bus: "virtio",
						},
					},
				},
				{
					Name:       "mydisk1",
					VolumeName: "nocloud",
					DiskDevice: v1.DiskDevice{
						Disk: &v1.DiskTarget{
							Bus: "virtio",
						},
					},
				},
				{
					Name:       "cdrom_tray_unspecified",
					VolumeName: "volume0",
					DiskDevice: v1.DiskDevice{
						CDRom: &v1.CDRomTarget{
							ReadOnly: &_false,
						},
					},
				},
				{
					Name:       "cdrom_tray_open",
					VolumeName: "volume1",
					DiskDevice: v1.DiskDevice{
						CDRom: &v1.CDRomTarget{
							Tray: v1.TrayStateOpen,
						},
					},
				},
				{
					Name:       "floppy_tray_unspecified",
					VolumeName: "volume2",
					DiskDevice: v1.DiskDevice{
						Floppy: &v1.FloppyTarget{},
					},
				},
				{
					Name:       "floppy_tray_open",
					VolumeName: "volume3",
					DiskDevice: v1.DiskDevice{
						Floppy: &v1.FloppyTarget{
							Tray:     v1.TrayStateOpen,
							ReadOnly: true,
						},
					},
				},
				{
					Name:       "should_default_to_disk",
					VolumeName: "volume4",
				},
				{
					Name:       "ephemeral_pvc",
					VolumeName: "volume5",
				},
			}
			vmi.Spec.Volumes = []v1.Volume{
				{
					Name: "myvolume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
						},
					},
				},
				{
					Name: "nocloud",
					VolumeSource: v1.VolumeSource{
						CloudInitNoCloud: &v1.CloudInitNoCloudSource{
							UserDataBase64: "1234",
						},
					},
				},
				{
					Name: "volume0",
					VolumeSource: v1.VolumeSource{
						CloudInitNoCloud: &v1.CloudInitNoCloudSource{
							UserDataBase64: "1234",
						},
					},
				},
				{
					Name: "volume1",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
						},
					},
				},
				{
					Name: "volume2",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
						},
					},
				},
				{
					Name: "volume3",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
						},
					},
				},
				{
					Name: "volume4",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
							ClaimName: "testclaim",
						},
					},
				},
				{
					Name: "volume5",
					VolumeSource: v1.VolumeSource{
						Ephemeral: &v1.EphemeralVolumeSource{
							PersistentVolumeClaim: &k8sv1.PersistentVolumeClaimVolumeSource{
								ClaimName: "testclaim",
							},
						},
					},
				},
			}
			vmi.Spec.Domain.Firmware = &v1.Firmware{
				UUID: "e4686d2c-6e8d-4335-b8fd-81bee22f4814",
			}

			gracePerod := int64(5)
			vmi.Spec.TerminationGracePeriodSeconds = &gracePerod

			vmi.ObjectMeta.UID = "f4686d2c-6e8d-4335-b8fd-81bee22f4814"
		})

		var convertedDomain = fmt.Sprintf(`<domain type="%s" xmlns:qemu="http://libvirt.org/schemas/domain/qemu/1.0">
  <name>mynamespace_testvmi</name>
  <memory unit="MB">9</memory>
  <os>
    <type arch="x86_64" machine="q35">hvmi</type>
  </os>
  <sysinfo type="smbios">
    <system>
      <entry name="uuid">e4686d2c-6e8d-4335-b8fd-81bee22f4814</entry>
    </system>
    <bios></bios>
    <baseBoard></baseBoard>
  </sysinfo>
  <devices>
    <interface type="bridge">
      <source bridge="br1"></source>
      <model type="virtio"></model>
    </interface>
    <video>
      <model type="vga" heads="1" vram="16384"></model>
    </video>
    <graphics type="vnc">
      <listen type="socket" socket="/var/run/kubevirt-private/mynamespace/testvmi/virt-vnc"></listen>
    </graphics>
    <disk device="disk" type="file">
      <source file="/var/run/kubevirt-private/vmi-disks/myvolume/disk.img"></source>
      <target bus="virtio" dev="vda"></target>
      <driver name="qemu" type="raw"></driver>
      <alias name="mydisk"></alias>
    </disk>
    <disk device="disk" type="file">
      <source file="/var/run/libvirt/cloud-init-dir/mynamespace/testvmi/noCloud.iso"></source>
      <target bus="virtio" dev="vdb"></target>
      <driver name="qemu" type="raw"></driver>
      <alias name="mydisk1"></alias>
    </disk>
    <disk device="cdrom" type="file">
      <source file="/var/run/libvirt/cloud-init-dir/mynamespace/testvmi/noCloud.iso"></source>
      <target bus="sata" dev="sda" tray="closed"></target>
      <driver name="qemu" type="raw"></driver>
      <alias name="cdrom_tray_unspecified"></alias>
    </disk>
    <disk device="cdrom" type="file">
      <source file="/var/run/kubevirt-private/vmi-disks/volume1/disk.img"></source>
      <target bus="sata" dev="sdb" tray="open"></target>
      <driver name="qemu" type="raw"></driver>
      <readonly></readonly>
      <alias name="cdrom_tray_open"></alias>
    </disk>
    <disk device="floppy" type="file">
      <source file="/var/run/kubevirt-private/vmi-disks/volume2/disk.img"></source>
      <target bus="fdc" dev="fda" tray="closed"></target>
      <driver name="qemu" type="raw"></driver>
      <alias name="floppy_tray_unspecified"></alias>
    </disk>
    <disk device="floppy" type="file">
      <source file="/var/run/kubevirt-private/vmi-disks/volume3/disk.img"></source>
      <target bus="fdc" dev="fdb" tray="open"></target>
      <driver name="qemu" type="raw"></driver>
      <readonly></readonly>
      <alias name="floppy_tray_open"></alias>
    </disk>
    <disk device="disk" type="file">
      <source file="/var/run/kubevirt-private/vmi-disks/volume4/disk.img"></source>
      <target bus="sata" dev="sdc"></target>
      <driver name="qemu" type="raw"></driver>
      <alias name="should_default_to_disk"></alias>
    </disk>
    <disk device="disk" type="file">
      <source file="/var/run/libvirt/kubevirt-ephemeral-disk/volume5/disk.qcow2"></source>
      <target bus="sata" dev="sdd"></target>
      <driver name="qemu" type="qcow2"></driver>
      <alias name="ephemeral_pvc"></alias>
      <backingStore type="file">
        <format type="raw"></format>
        <source file="/var/run/kubevirt-private/vmi-disks/volume5/disk.img"></source>
      </backingStore>
    </disk>
    <serial type="unix">
      <target port="0"></target>
      <source mode="bind" path="/var/run/kubevirt-private/mynamespace/testvmi/virt-serial0"></source>
    </serial>
    <console type="pty">
      <target type="serial" port="0"></target>
    </console>
    <watchdog model="i6300esb" action="poweroff">
      <alias name="mywatchdog"></alias>
    </watchdog>
  </devices>
  <clock offset="utc" adjustment="reset">
    <timer name="rtc" tickpolicy="catchup" present="yes" track="guest"></timer>
    <timer name="pit" tickpolicy="discard" present="no"></timer>
    <timer name="kvmclock" present="yes"></timer>
    <timer name="hpet" tickpolicy="delay" present="no"></timer>
    <timer name="hypervclock" present="yes"></timer>
  </clock>
  <metadata>
    <kubevirt xmlns="http://kubevirt.io">
      <uid>f4686d2c-6e8d-4335-b8fd-81bee22f4814</uid>
      <graceperiod>
        <deletionGracePeriodSeconds>5</deletionGracePeriodSeconds>
      </graceperiod>
    </kubevirt>
  </metadata>
  <features>
    <acpi></acpi>
    <apic></apic>
    <hyperv>
      <relaxed state="off"></relaxed>
      <vapic state="on"></vapic>
      <spinlocks state="on" retries="4096"></spinlocks>
      <vpindex state="on"></vpindex>
      <runtime state="off"></runtime>
      <synic state="on"></synic>
      <stimer state="off"></stimer>
      <reset state="on"></reset>
      <vendor_id state="off" value="myvendor"></vendor_id>
    </hyperv>
  </features>
  <cpu></cpu>
</domain>`, domainType)

		var c *ConverterContext

		BeforeEach(func() {
			c = &ConverterContext{
				VirtualMachine: vmi,
				Secrets: map[string]*k8sv1.Secret{
					"mysecret": {
						Data: map[string][]byte{
							"node.session.auth.username": []byte("admin"),
						},
					},
				},
				AllowEmulation: true,
			}
		})

		It("should be converted to a libvirt Domain with vmi defaults set", func() {
			v1.SetObjectDefaults_VirtualMachineInstance(vmi)
			Expect(vmiToDomainXML(vmi, c)).To(Equal(convertedDomain))
		})

		It("should use kvm if present", func() {
			v1.SetObjectDefaults_VirtualMachineInstance(vmi)
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).Type).To(Equal(domainType))
		})

		It("should convert CPU cores", func() {
			v1.SetObjectDefaults_VirtualMachineInstance(vmi)
			vmi.Spec.Domain.CPU = &v1.CPU{
				Cores: 3,
			}
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).CPU.Topology.Cores).To(Equal(uint32(3)))
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).CPU.Topology.Sockets).To(Equal(uint32(1)))
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).CPU.Topology.Threads).To(Equal(uint32(1)))
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).VCPU.Placement).To(Equal("static"))
			Expect(vmiToDomainXMLToDomainSpec(vmi, c).VCPU.CPUs).To(Equal(uint32(3)))

		})

		It("should select explicitly chosen network model", func() {
			v1.SetObjectDefaults_VirtualMachineInstance(vmi)
			vmi.ObjectMeta.Annotations = map[string]string{v1.InterfaceModel: "e1000"}
			domain := vmiToDomain(vmi, c)
			Expect(domain.Spec.Devices.Interfaces[0].Model.Type).To(Equal("e1000"))
		})
	})
})

func diskToDiskXML(disk *v1.Disk) string {
	devicePerBus := make(map[string]int)
	libvirtDisk := &Disk{}
	Expect(Convert_v1_Disk_To_api_Disk(disk, libvirtDisk, devicePerBus)).To(Succeed())
	data, err := xml.MarshalIndent(libvirtDisk, "", "  ")
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}

func vmiToDomainXML(vmi *v1.VirtualMachineInstance, c *ConverterContext) string {
	domain := vmiToDomain(vmi, c)
	data, err := xml.MarshalIndent(domain.Spec, "", "  ")
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}

func vmiToDomain(vmi *v1.VirtualMachineInstance, c *ConverterContext) *Domain {
	domain := &Domain{}
	Expect(Convert_v1_VirtualMachine_To_api_Domain(vmi, domain, c)).To(Succeed())
	SetObjectDefaults_Domain(domain)
	return domain
}

func xmlToDomainSpec(data string) *DomainSpec {
	newDomain := &DomainSpec{}
	err := xml.Unmarshal([]byte(data), newDomain)
	newDomain.XMLName.Local = ""
	newDomain.XmlNS = "http://libvirt.org/schemas/domain/qemu/1.0"
	Expect(err).To(BeNil())
	return newDomain
}

func vmiToDomainXMLToDomainSpec(vmi *v1.VirtualMachineInstance, c *ConverterContext) *DomainSpec {
	return xmlToDomainSpec(vmiToDomainXML(vmi, c))
}
