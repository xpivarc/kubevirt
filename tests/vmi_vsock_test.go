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
 * Copyright 2022 Red Hat, Inc.
 *
 */

package tests_test

import (
	"encoding/xml"
	"net"
	"os"
	"time"

	"kubevirt.io/kubevirt/tests/libmigration"

	"kubevirt.io/kubevirt/tests/decorators"

	expect "github.com/google/goexpect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/tests/libssh"

	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/tests/flags"
	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/libvmi"

	"kubevirt.io/client-go/kubecli"

	"kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/console"
)

var _ = FDescribe("[sig-compute]VSOCK", decorators.SigCompute, decorators.VSOCK, func() {
	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		virtClient = kubevirt.Client()
	})

	withAutoAttachVSCOK := func(vmi *v1.VirtualMachineInstance) {
		vmi.Spec.Domain.Devices.AutoattachVSOCK = pointer.Bool(true)
	}

	Context("VM creation", func() {
		DescribeTable("should expose a VSOCK device", func(useVirtioTransitional bool) {
			useVirtioTransitionalOption := func(vmi *v1.VirtualMachineInstance) {
				vmi.Spec.Domain.Devices.UseVirtioTransitional = &useVirtioTransitional
			}

			By("Creating a VMI with VSOCK enabled")
			vmi := libvmi.NewFedora(
				useVirtioTransitionalOption,
				withAutoAttachVSCOK,
			)

			vmi = tests.RunVMIAndExpectLaunch(vmi, 60)
			Expect(vmi.Status.VSOCKCID).NotTo(BeNil())

			assertCIDIsAssigned(virtClient, vmi)

			By("Logging in as root")
			Expect(console.LoginToFedora(vmi)).To(Succeed())

			By("Ensuring a vsock device is present")
			Expect(console.SafeExpectBatch(vmi, []expect.Batcher{
				&expect.BSnd{S: "ls /dev/vsock-vhost\n"},
				&expect.BExp{R: "/dev/vsock-vhost"},
			}, 300)).To(Succeed(), "Could not find a vsock-vhost device")
			Expect(console.SafeExpectBatch(vmi, []expect.Batcher{
				&expect.BSnd{S: "ls /dev/vsock\n"},
				&expect.BExp{R: "/dev/vsock"},
			}, 300)).To(Succeed(), "Could not find a vsock device")
		},
			Entry("Use virtio transitional", true),
			Entry("Use virtio non-transitional", false),
		)
	})

	Context("Live migration", func() {

		It("should retain the CID for migration target", func() {
			By("Creating a VMI with VSOCK enabled")
			vmi := libvmi.NewFedora(
				append(libvmi.WithMasqueradeNetworking(), withAutoAttachVSCOK)...,
			)
			vmi = tests.RunVMIAndExpectLaunch(vmi, 60)
			Expect(vmi.Status.VSOCKCID).NotTo(BeNil())

			assertCIDIsAssigned(virtClient, vmi)

			By("Creating a new VMI with VSOCK enabled on the same node")
			vmi2 := libvmi.NewFedora(
				append(libvmi.WithMasqueradeNetworking(),
					withAutoAttachVSCOK, libvmi.WithNodeAffinityFor(vmi.Status.NodeName))...,
			)
			vmi2 = tests.RunVMIAndExpectLaunch(vmi2, 60)
			Expect(vmi2.Status.VSOCKCID).NotTo(BeNil())

			assertCIDIsAssigned(virtClient, vmi2)

			By("Migrating the 2nd VMI")
			libmigration.RunMigrationAndExpectToCompleteWithDefaultTimeout(virtClient,
				tests.NewRandomMigration(vmi2.Name, vmi2.Namespace),
			)

			assertCIDIsAssigned(virtClient, vmi2)
		})
	})

	DescribeTable("communicating with VMI via VSOCK", func(useTLS bool) {
		if flags.KubeVirtExampleGuestAgentPath == "" {
			Skip("example guest agent path is not specified")
		}
		privateKeyPath, publicKey, err := libssh.GenerateKeyPair(GinkgoT().TempDir())
		Expect(err).ToNot(HaveOccurred())

		userData := libssh.RenderUserDataWithKey(publicKey)
		vmi := libvmi.NewFedora(
			libvmi.WithInterface(libvmi.InterfaceDeviceWithMasqueradeBinding()),
			libvmi.WithNetwork(v1.DefaultPodNetwork()),
			libvmi.WithCloudInitNoCloudUserData(userData, false),
			withAutoAttachVSCOK,
		)
		vmi = tests.RunVMIAndExpectLaunch(vmi, 60)

		By("Logging in as root")
		Expect(console.LoginToFedora(vmi)).To(Succeed())

		By("copying the guest agent binary")
		Expect(os.Setenv("SSH_AUTH_SOCK", "/dev/null")).To(Succeed())
		Expect(libssh.SCPToVMI(vmi, privateKeyPath, flags.KubeVirtExampleGuestAgentPath, "/usr/bin/")).To(Succeed())

		By("starting the guest agent binary")
		Expect(tests.StartExampleGuestAgent(vmi, useTLS, 1234)).To(Succeed())
		time.Sleep(2 * time.Second)

		By("Connect to the guest via API")
		cliConn, svrConn := net.Pipe()
		defer func() {
			_ = cliConn.Close()
			_ = svrConn.Close()
		}()
		stopChan := make(chan error)
		go func() {
			defer GinkgoRecover()
			vsock, err := virtClient.VirtualMachineInstance(vmi.Namespace).VSOCK(vmi.Name, &v1.VSOCKOptions{TargetPort: uint32(1234), UseTLS: pointer.Bool(useTLS)})
			if err != nil {
				stopChan <- err
				return
			}
			stopChan <- vsock.Stream(kubecli.StreamOptions{
				In:  svrConn,
				Out: svrConn,
			})
		}()

		Expect(cliConn.SetDeadline(time.Now().Add(10 * time.Second))).To(Succeed())

		By("Writing to the Guest")
		message := "Hello World?"
		_, err = cliConn.Write([]byte(message))
		Expect(err).NotTo(HaveOccurred())

		By("Reading from the Guest")
		buf := make([]byte, 1024)
		n, err := cliConn.Read(buf)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(buf[0:n])).To(Equal(message))

		select {
		case err := <-stopChan:
			Expect(err).NotTo(HaveOccurred())
		default:
		}
	},
		Entry("should succeed with TLS on both sides", true),
		Entry("should succeed without TLS on both sides", false),
	)

	It("should return err if the port is invalid", func() {
		By("Creating a VMI with VSOCK enabled")
		vmi := libvmi.NewFedora(withAutoAttachVSCOK)
		vmi = tests.RunVMIAndExpectLaunch(vmi, 60)

		By("Connect to the guest on invalide port")
		_, err := virtClient.VirtualMachineInstance(vmi.Namespace).VSOCK(vmi.Name, &v1.VSOCKOptions{TargetPort: uint32(0)})
		Expect(err).To(MatchError("target port is required but not provided"))
	})

	It("should return err if no app listerns on the port", func() {
		By("Creating a VMI with VSOCK enabled")
		vmi := libvmi.NewFedora(withAutoAttachVSCOK)
		vmi = tests.RunVMIAndExpectLaunch(vmi, 60)

		By("Connect to the guest on the unused port")
		cliConn, svrConn := net.Pipe()
		defer func() {
			_ = cliConn.Close()
			_ = svrConn.Close()
		}()
		vsock, err := virtClient.VirtualMachineInstance(vmi.Namespace).VSOCK(vmi.Name, &v1.VSOCKOptions{TargetPort: uint32(9999)})
		Expect(err).NotTo(HaveOccurred())
		Expect(vsock.Stream(kubecli.StreamOptions{
			In:  svrConn,
			Out: svrConn,
		})).To(MatchError(ContainSubstring("EOF")))
	})
})

func assertCIDIsAssigned(virtClient kubecli.KubevirtClient, vmi *v1.VirtualMachineInstance) {
	By("creating valid libvirt domain")
	domain, err := tests.GetRunningVirtualMachineInstanceDomainXML(virtClient, vmi)
	Expect(err).ToNot(HaveOccurred())
	spec := &api.DomainSpec{}
	Expect(xml.Unmarshal([]byte(domain), spec)).To(Succeed())
	Expect(spec.Devices.VSOCK.CID.Auto).To(Equal("no"))
	Expect(spec.Devices.VSOCK.CID.Address).To(Equal(*vmi.Status.VSOCKCID))

}
