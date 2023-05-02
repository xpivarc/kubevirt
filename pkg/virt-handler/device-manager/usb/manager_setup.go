package usb

import "kubevirt.io/kubevirt/pkg/controller"

func (manager *realUSBManager) addFunc(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err == nil {
		manager.queue.Add(key)
	}
}

func (manager *realUSBManager) updateFunc(_, updatedObj interface{}) {
	key, err := controller.KeyFunc(updatedObj)
	if err == nil {
		manager.queue.Add(key)
	}
}

func (manager *realUSBManager) deleteFunc(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err == nil {
		manager.queue.Add(key)
	}
}
