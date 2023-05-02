package usb

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubevirt.io/api/usb/v1alpha1"
	"kubevirt.io/client-go/log"
)

type USBManager interface {
	Run(threadiness int, stopCh chan struct{})
}
type realUSBManager struct {
	nodeConfigInformer cache.SharedIndexInformer
	queue              workqueue.RateLimitingInterface
}

func NewUSBManager(nodeConfigInformer cache.SharedIndexInformer, queue workqueue.RateLimitingInterface) USBManager {
	manager := &realUSBManager{
		nodeConfigInformer: nodeConfigInformer,
		queue:              queue,
	}
	nodeConfigInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addFunc,
		UpdateFunc: manager.updateFunc,
		DeleteFunc: manager.deleteFunc,
	})
	return manager
}

func (manager *realUSBManager) Run(threadiness int, stopCh chan struct{}) {
	defer manager.queue.ShutDown()

	log.Log.Info("Starting USB manager")

	cache.WaitForCacheSync(stopCh, manager.nodeConfigInformer.HasSynced)

	// Start the actual work
	for i := 0; i < threadiness; i++ {
		go wait.Until(manager.runWorker, time.Second, stopCh)
	}
	log.Log.Info("Started USB manager")

	<-stopCh
	log.Log.Info("Stoping USB manager")
}

func (manager *realUSBManager) Execute() bool {
	key, quit := manager.queue.Get()
	if quit {
		return false
	}
	defer manager.queue.Done(key)
	if err := manager.execute(key.(string)); err != nil {
		log.Log.Reason(err).Infof("re-enqueuing NodeConfig %v", key)
		manager.queue.AddRateLimited(key)
	} else {
		log.Log.V(4).Infof("processed NodeConfig %v", key)
		manager.queue.Forget(key)
	}
	return true
}

func (manager *realUSBManager) runWorker() {
	for manager.Execute() {
	}
}

func (manager *realUSBManager) execute(key string) error {
	obj, exists, err := manager.nodeConfigInformer.GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get object for key %s, %v", key, err)
	}
	if !exists {
		// todo clean old conf?
		return nil
	}

	nodeConfig := obj.(*v1alpha1.NodeConfig)
	return manager.syncDevicePlugin(nodeConfig)
}

func (manager *realUSBManager) syncDevicePlugin(nodeConfig *v1alpha1.NodeConfig) error {
	// TODO here we need to start the device plugin for each
	// resource we want to expose (e.g webcam, weathercam, termostat) with
	// usb device nodes that corresponds to the resource

	// call NewUSBDevicePlugin
	return nil
}
