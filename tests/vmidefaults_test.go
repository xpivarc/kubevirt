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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/kubevirt/tests/decorators"

	"kubevirt.io/kubevirt/tests/framework/kubevirt"

	"kubevirt.io/client-go/kubecli"
)

var _ = Describe("[Serial][sig-compute]VMIDefaults", Serial, decorators.SigCompute, func() {
	var virtClient kubecli.KubevirtClient

	// TODO create one test to verify webhook is called and some mutating is done

	BeforeEach(func() {
		virtClient = kubevirt.Client()
		Expect(virtClient).ToNot(BeEmpty())
	})

})
