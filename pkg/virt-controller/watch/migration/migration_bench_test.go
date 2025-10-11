package migration

import (
	"fmt"
	"testing"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/controller"
	"kubevirt.io/kubevirt/pkg/testutils"
)

func setup(podCount int) (*Controller, *v1.VirtualMachineInstance, *v1.VirtualMachineInstanceMigration, *k8sv1.Pod) {
	informer, _ := testutils.NewFakeInformerFor(&k8sv1.Pod{})
	controller := Controller{
		podIndexer: informer.GetIndexer(),
	}

	if err := addTargetPodIndexer(controller.podIndexer); err != nil {
		panic(err)
	}

	migration := v1.VirtualMachineInstanceMigration{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("something"),
			Namespace: "vm-debug",
		},
	}

	vmi := v1.VirtualMachineInstance{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("something"),
			Namespace: "vm-debug",
		},
	}

	for i := range podCount - 1 {
		pod := k8sv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID(fmt.Sprintf("something%d", i)),
				Namespace: "vm-debug",
				Name:      fmt.Sprintf("name%d", i),
				Labels: map[string]string{
					"kubevirt.io":                         "virt-launcher",
					"kubevirt.io/created-by":              "b0c482c0-0e0d-40b9-8dff-de967de41e5a",
					"kubevirt.io/domain":                  "rhel9",
					"kubevirt.io/migrationJobUID":         "815e781c-44f6-4ae7-9403-dc156cebd1fd",
					"kubevirt.io/migrationTargetNodeName": "e28-h03-000-r650",
					"kubevirt.io/nodeName":                "e28-h03-000-r650",
					"vm.kubevirt.io/name":                 "rhel9-2236",
				},
			},
		}
		err := controller.podIndexer.Add(&pod)
		if err != nil {
			panic(err)
		}
	}

	pod := k8sv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("something"),
			Namespace: "vm-debug",
			Name:      "name",
			Labels: map[string]string{
				"kubevirt.io":                         "virt-launcher",
				"kubevirt.io/created-by":              string(vmi.UID),
				"kubevirt.io/domain":                  "rhel9",
				"kubevirt.io/migrationJobUID":         string(migration.UID),
				"kubevirt.io/migrationTargetNodeName": "e28-h03-000-r650",
				"kubevirt.io/nodeName":                "e28-h03-000-r650",
				"vm.kubevirt.io/name":                 "rhel9-2236",
			},
		},
	}

	err := controller.podIndexer.Add(&pod)
	if err != nil {
		panic(err)
	}

	return &controller, &vmi, &migration, &pod

}
func BenchmarkListMatchingTargetPods1000(b *testing.B) {
	benchmarkListMatchingTargetPods(b, 1000)
}

func BenchmarkListMatchingTargetPods10000(b *testing.B) {
	benchmarkListMatchingTargetPods(b, 10000)
}

func benchmarkListMatchingTargetPods(b *testing.B, count int) {
	controller, vmi, migration, pod := setup(count)

	b.ResetTimer()

	for b.Loop() {
		pods, err := controller.listMatchingTargetPods(migration, vmi)
		if err != nil {
			panic(err)
		}
		if len(pods) != 1 {
			panic("hi")

		}
		if pods[0] != pod {
			panic("hi")
		}
	}

}
func BenchmarkListMatchingTargetPodsParallel1000(b *testing.B) {
	benchmarkListMatchingTargetPodsParallel(b, 1000)
}
func BenchmarkListMatchingTargetPodsParallel10000(b *testing.B) {
	benchmarkListMatchingTargetPodsParallel(b, 10000)
}

func benchmarkListMatchingTargetPodsParallel(b *testing.B, count int) {
	controller, vmi, migration, pod := setup(count)

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			pods, err := controller.listMatchingTargetPods(migration, vmi)
			if err != nil {
				panic(err)
			}
			if len(pods) != 1 {
				panic("hi")

			}
			if pods[0] != pod {
				panic("hi")
			}
		}
	})
}

func setup2(migCount int) *Controller {
	informer, _ := testutils.NewFakeInformerFor(&k8sv1.Pod{})
	informerMig, _ := testutils.NewFakeInformerFor(&v1.VirtualMachineInstanceMigration{})
	informerVMI, _ := testutils.NewFakeInformerFor(&v1.VirtualMachineInstance{})
	c := Controller{
		podIndexer:       informer.GetIndexer(),
		migrationIndexer: informerMig.GetIndexer(),
		vmiStore:         informerVMI.GetStore(),
	}

	indexer := controller.GetVirtualMachineInstanceMigrationInformerIndexers()
	delete(indexer, cache.NamespaceIndex)
	err := c.migrationIndexer.AddIndexers(indexer)
	if err != nil {
		panic(err)
	}

	for i := range migCount {
		vmi := v1.VirtualMachineInstance{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID(fmt.Sprintf("something%d", i)),
				Namespace: "vm-debug",
				Name:      fmt.Sprintf("name%d", i),
			},
		}
		err := c.vmiStore.Add(&vmi)
		if err != nil {
			panic(err)
		}

		migration := v1.VirtualMachineInstanceMigration{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID(fmt.Sprintf("something%d", i)),
				Namespace: "vm-debug",
				Name:      fmt.Sprintf("name%d", i),
			},
			Spec: v1.VirtualMachineInstanceMigrationSpec{
				VMIName: vmi.Name,
			},
		}
		err = c.migrationIndexer.Add(&migration)
		if err != nil {
			panic(err)
		}

		pod := k8sv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				UID:       types.UID(fmt.Sprintf("something%d", i)),
				Namespace: "vm-debug",
				Name:      fmt.Sprintf("name%d", i),
				Labels: map[string]string{
					"kubevirt.io":            "virt-launcher",
					"kubevirt.io/created-by": string(vmi.UID),
					"kubevirt.io/domain":     "rhel9",
					// "kubevirt.io/migrationJobUID":         string(migration.UID),
					// "kubevirt.io/migrationTargetNodeName": "e28-h03-000-r650",
					"kubevirt.io/nodeName": "e28-h03-000-r650",
					"vm.kubevirt.io/name":  "rhel9-2236",
				},
			},
		}
		err = c.podIndexer.Add(&pod)
		if err != nil {
			panic(err)
		}
	}
	return &c
}

func BenchmarkFindRunningMigrations1000(b *testing.B) {
	benchmarkFindRunningMigrations(b, 1000)
}

func BenchmarkFindRunningMigrations10000(b *testing.B) {
	benchmarkFindRunningMigrations(b, 10000)
}

func benchmarkFindRunningMigrations(b *testing.B, migCount int) {
	c := setup2(migCount)

	b.ResetTimer()

	for b.Loop() {
		migrations, err := c.findRunningMigrations()
		if err != nil {
			panic(err)
		}
		if len(migrations) != 0 {
			panic("hi")
		}
	}
}
