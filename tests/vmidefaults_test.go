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
 * Copyright 2018 Red Hat, Inc.
 *
 */

package tests_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"kubevirt.io/kubevirt/tests/decorators"

	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/libvmi"
	"kubevirt.io/kubevirt/tests/libwait"
	"kubevirt.io/kubevirt/tests/testsuite"
	"kubevirt.io/kubevirt/tests/util"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/tests"
)

var _ = Describe("[Serial][sig-compute]VMIDefaults", Serial, decorators.SigCompute, func() {
	var err error
	var virtClient kubecli.KubevirtClient

	var vmi *v1.VirtualMachineInstance

	BeforeEach(func() {
		virtClient = kubevirt.Client()
	})

	Context("MemBalloon defaults", func() {
		var kvConfiguration v1.KubeVirtConfiguration

		BeforeEach(func() {
			// create VMI with missing disk target
			vmi = tests.NewRandomVMI()

			kv := util.GetCurrentKv(virtClient)
			kvConfiguration = kv.Spec.Configuration
		})

		It("[test_id:4556]Should be present in domain", func() {
			By("Creating a virtual machine")
			vmi, err = virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(nil)).Create(context.Background(), vmi)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for successful start")
			libwait.WaitForSuccessfulVMIStart(vmi)

			By("Getting domain of vmi")
			domain, err := tests.GetRunningVMIDomainSpec(vmi)
			Expect(err).ToNot(HaveOccurred())

			expected := api.MemBalloon{
				Model: "virtio-non-transitional",
				Stats: &api.Stats{
					Period: 10,
				},
				Address: &api.Address{
					Type:     api.AddressPCI,
					Domain:   "0x0000",
					Bus:      "0x07",
					Slot:     "0x00",
					Function: "0x0",
				},
			}
			if kvConfiguration.VirtualMachineOptions != nil && kvConfiguration.VirtualMachineOptions.DisableFreePageReporting != nil {
				expected.FreePageReporting = "off"
			} else {
				expected.FreePageReporting = "on"
			}
			Expect(domain.Devices.Ballooning).ToNot(BeNil(), "There should be default memballoon device")
			Expect(*domain.Devices.Ballooning).To(Equal(expected), "Default to virtio model and 10 seconds pooling")
		})

	})

	Context("Input defaults", func() {

		It("[test_id:TODO]Should be applied to a device added by AutoattachInputDevice", func() {
			By("Creating a VirtualMachine with AutoattachInputDevice enabled")
			vm := tests.NewRandomVirtualMachine(libvmi.NewCirros(), false)
			vm.Spec.Template.Spec.Domain.Devices.AutoattachInputDevice = pointer.Bool(true)
			vm, err = virtClient.VirtualMachine(testsuite.GetTestNamespace(nil)).Create(context.Background(), vm)
			Expect(err).ToNot(HaveOccurred())

			By("Starting VirtualMachine")
			vm = tests.StartVMAndExpectRunning(virtClient, vm)

			By("Getting VirtualMachineInstance")
			vmi, err = virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Get(context.Background(), vm.Name, &metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(vmi.Spec.Domain.Devices.Inputs).ToNot(BeEmpty(), "There should be input devices")
			Expect(vmi.Spec.Domain.Devices.Inputs[0].Name).To(Equal("default-0"))
			Expect(vmi.Spec.Domain.Devices.Inputs[0].Type).To(Equal(v1.InputTypeTablet))
			Expect(vmi.Spec.Domain.Devices.Inputs[0].Bus).To(Equal(v1.InputBusUSB))
		})

	})
})
