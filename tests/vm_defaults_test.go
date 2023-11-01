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
	"kubevirt.io/kubevirt/tests/testsuite"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/tests"
)

var _ = Describe("[sig-compute]VM Defaults", decorators.SigCompute, func() {

	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		virtClient = kubevirt.Client()
	})

	Context("Input defaults", func() {

		It("[test_id:TODO]Should be applied to a device added by AutoattachInputDevice", func() {
			By("Creating a VirtualMachine with AutoattachInputDevice enabled")
			vm := tests.NewRandomVirtualMachine(libvmi.NewCirros(), false)
			vm.Spec.Template.Spec.Domain.Devices.AutoattachInputDevice = pointer.Bool(true)
			vm, err := virtClient.VirtualMachine(testsuite.GetTestNamespace(nil)).Create(context.Background(), vm)
			Expect(err).ToNot(HaveOccurred())

			By("Starting VirtualMachine")
			vm = tests.StartVMAndExpectRunning(virtClient, vm)

			By("Getting VirtualMachineInstance")
			vmi, err := virtClient.VirtualMachineInstance(testsuite.GetTestNamespace(vm)).Get(context.Background(), vm.Name, &metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			inputs := vmi.Spec.Domain.Devices.Inputs

			Expect(inputs).ToNot(BeEmpty(), "There should be input devices")
			Expect(inputs[0].Name).To(Equal("default-0"))
			Expect(inputs[0].Type).To(Equal(v1.InputTypeTablet))
			Expect(inputs[0].Bus).To(Equal(v1.InputBusUSB))
		})

	})
})
