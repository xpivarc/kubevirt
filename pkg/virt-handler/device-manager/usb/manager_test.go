package usb

import (
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	framework "k8s.io/client-go/tools/cache/testing"
	"kubevirt.io/api/usb/v1alpha1"
	"kubevirt.io/kubevirt/pkg/testutils"
)

type nodeConfigFeeder struct {
	MockQueue *testutils.MockWorkQueue
	Source    *framework.FakeControllerSource
}

func (v *nodeConfigFeeder) Add(vmi *v1alpha1.NodeConfig) {
	v.MockQueue.ExpectAdds(1)
	v.Source.Add(vmi)
	v.MockQueue.Wait()
}

func (v *nodeConfigFeeder) Modify(vmi *v1alpha1.NodeConfig) {
	v.MockQueue.ExpectAdds(1)
	v.Source.Modify(vmi)
	v.MockQueue.Wait()
}

func (v *nodeConfigFeeder) Delete(vmi *v1alpha1.NodeConfig) {
	v.MockQueue.ExpectAdds(1)
	v.Source.Delete(vmi)
	v.MockQueue.Wait()
}

func newNodeConfigFeeder(queue *testutils.MockWorkQueue, source *framework.FakeControllerSource) *nodeConfigFeeder {
	return &nodeConfigFeeder{
		MockQueue: queue,
		Source:    source,
	}
}

var _ = Describe("USB Manager", func() {
	var (
		manager *USBManager
		feeder  *nodeConfigFeeder
	)

	BeforeEach(func() {
		informer, source := testutils.NewFakeInformerFor(&v1alpha1.NodeConfig{})
		manager = NewUSBManager(informer)
		queue := testutils.NewMockWorkQueue(manager.queue)
		manager.queue = queue
		feeder = newNodeConfigFeeder(queue, source)
		stop := make(chan struct{})
		DeferCleanup(func() { close(stop) })
		go informer.Run(stop)
		Expect(cache.WaitForCacheSync(stop, informer.HasSynced)).To(BeTrue())
	})

	Context("sanity test", func() {
		BeforeEach(func() {
			feeder.Add(&v1alpha1.NodeConfig{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test",
					Name:      "test",
				},
				Spec: v1alpha1.NodeConfigSpec{
					USB: []v1alpha1.USB{
						{
							ResourceName: "kubevirt.io/usb-storage",
							USBHostDevices: []v1alpha1.USBHostDevices{
								{
									SelectByVendorProduct: "145f:019f",
								},
							},
						},
					},
				},
			})
		})

		It("should start plugin", func() {
			manager.discoveryFunc = func() []*usbDevice {
				vendor, _ := strconv.ParseInt("145f", 16, 32)
				product, _ := strconv.ParseInt("019f", 16, 32)

				return []*usbDevice{
					{
						Vendor:  int(vendor),
						Product: int(product),
					},
				}
			}
			manager.Execute()
			Expect(manager.handlers).To(HaveKey("kubevirt.io/usb-storage"))
		})

	})
})
