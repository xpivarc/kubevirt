package usb

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubevirt.io/api/usb/v1alpha1"
	"kubevirt.io/client-go/log"
)

type USBManagerInterface interface {
	Run(stopCh chan struct{})
}

type USBManager struct {
	nodeConfigInformer cache.SharedIndexInformer
	queue              workqueue.RateLimitingInterface
	discoveryFunc      func() []*usbDevice
	handlers           map[string]pluginHandler
	handlersLock       sync.Mutex
}

type pluginHandler struct {
	started  bool
	failed   bool
	stopChan chan struct{}
	plugin   Plugin
}

type usbDeviceSelector struct {
	resourceName string
	vendor       int
	product      int
}

func NewUSBManager(nodeConfigInformer cache.SharedIndexInformer) *USBManager {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "virt-handler-nodeconfig")
	manager := &USBManager{
		nodeConfigInformer: nodeConfigInformer,
		queue:              queue,
		handlers:           make(map[string]pluginHandler),
		discoveryFunc:      discoverUSBDevices,
	}
	nodeConfigInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    manager.addFunc,
		UpdateFunc: manager.updateFunc,
		DeleteFunc: manager.deleteFunc,
	})
	return manager
}

func (manager *USBManager) Run(stopCh chan struct{}) {
	defer manager.queue.ShutDown()

	log.Log.Info("Starting USB manager")

	cache.WaitForCacheSync(stopCh, manager.nodeConfigInformer.HasSynced)

	// Start the actual work
	go wait.Until(manager.runWorker, time.Second, stopCh)
	log.Log.Info("Started USB manager")

	<-stopCh
	log.Log.Info("Stoping USB manager")
}

func (manager *USBManager) Execute() bool {
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

func (manager *USBManager) runWorker() {
	for manager.Execute() {
	}
}

func (manager *USBManager) execute(key string) error {
	obj, exists, err := manager.nodeConfigInformer.GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get object for key %s, %v", key, err)
	}

	if !exists || obj == nil {
		// TODO clean old conf?
		return nil
	}

	nodeConfig := obj.(*v1alpha1.NodeConfig)
	return manager.syncDevicePlugin(nodeConfig)
}

func constructPermittedUSBDevicesMap(nodeConfig *v1alpha1.NodeConfig) map[int][]usbDeviceSelector {
	// Iterate over requested USB Devices and map it vendor:product
	permittedUSBDevices := make(map[int][]usbDeviceSelector)
	for indexUsbType, usbType := range nodeConfig.Spec.USB {
		resourceName := usbType.ResourceName
		log.Log.Infof("Iterating over %s, at %d with %d usb host devices",
			resourceName, indexUsbType, len(usbType.USBHostDevices))
		for index, dev := range usbType.USBHostDevices {
			sep := strings.Index(dev.SelectByVendorProduct, ":")
			if sep == -1 {
				log.Log.Warningf("Failed to parse USBHostDevices[%d] = %s",
					index, dev.SelectByVendorProduct)
				continue
			}
			val, err := strconv.ParseInt(dev.SelectByVendorProduct[:sep], 16, 32)
			if err != nil {
				log.Log.Warningf("Failed to convert vendor from base16 string to int: %s",
					dev.SelectByVendorProduct[:sep])
				continue
			}
			vendor := int(val)

			val, err = strconv.ParseInt(dev.SelectByVendorProduct[sep+1:], 16, 32)
			if err != nil {
				log.Log.Warningf("Failed to convert product from base16 string to int: %s",
					dev.SelectByVendorProduct[:sep])
				continue
			}
			product := int(val)

			permittedUSBDevices[vendor] = append(permittedUSBDevices[vendor],
				usbDeviceSelector{
					resourceName: resourceName,
					vendor:       vendor,
					product:      product,
				})
		}
	}
	return permittedUSBDevices
}

func (manager *USBManager) syncDevicePlugin(nodeConfig *v1alpha1.NodeConfig) error {
	log.Log.Infof("%s sync", nodeConfig.Name)

	// Sanity check
	if nodeConfig == nil || len(nodeConfig.Spec.USB) == 0 {
		log.Log.V(5).Infof("No USB devices")
		return nil
	}
	localDevicesFound := manager.discoveryFunc()
	if len(localDevicesFound) == 0 {
		log.Log.V(5).Info("No USB devices found in this node")
		return nil
	}

	permittedDevicesPerVendor := constructPermittedUSBDevicesMap(nodeConfig)

	devicesToExport := map[string][]*usbDevice{}
	for _, device := range localDevicesFound {
		permittedDevices, vendorMatched := permittedDevicesPerVendor[device.Vendor]
		if !vendorMatched {
			continue
		}
		for _, permpermittedDevice := range permittedDevices {
			if permpermittedDevice.product != device.Product {
				continue
			}

			resourceName := permpermittedDevice.resourceName

			_, ok := devicesToExport[resourceName]
			if !ok {
				devicesToExport[resourceName] = []*usbDevice{}
			}
			devicesToExport[resourceName] = append(devicesToExport[resourceName], device)
		}
	}
	// TODO here we need to start the device plugin for each
	// resource we want to expose (e.g webcam, weathercam, termostat) with
	// usb device nodes that corresponds to the resource

	// TODO: What should I do with Plugin?
	// This is called several times, I suppose I should store and manage this myself.
	// The USB Manager knows that Plugin is start/stop and does updates based on k8s
	// changes (e.g: changed NodeConfig)

	// Do I have a device plugin already for this resource name?
	// if yes - is it in sync? did we remove or added devices by modifying the selecotr
	// if not - create new plugin

	for resourceName, devices := range devicesToExport {
		manager.startPlugin(NewUSBDevicePlugin(resourceName, devices))
	}
	return nil
}

func (manager *USBManager) startPlugin(plugin Plugin) {

	// TODO reduce
	manager.handlersLock.Lock()
	defer manager.handlersLock.Unlock()
	if _, ok := manager.handlers[plugin.Name()]; ok {
		log.Log.V(9).Infof("USB pluggin %s is already started", plugin.Name())
		return
	}

	log.Log.Infof("USB pluggin %s starting", plugin.Name())

	handler := pluginHandler{
		stopChan: make(chan struct{}),
		plugin:   plugin,
	}

	logger := log.DefaultLogger()
	go func() {
		retries := 0
		for {
			err := plugin.Start(handler.stopChan)
			if err == nil {
				handler.started = true
				logger.Reason(err).Infof("Started %s USB pluggin.", plugin.Name())
				return
			}
			retries++
			if retries > 10 {
				logger.Reason(err).Errorf("Unable to start %s USB pluggin", plugin.Name())
				handler.failed = true
				return
			}

			logger.Reason(err).Errorf("Error starting %s USB pluggin. Retry #%d",
				plugin.Name(), retries)

			select {
			case <-handler.stopChan:
				// Start has been cancelled
				return
			case <-time.After(10 * time.Second):
				// Try again
				continue
			}
		}
	}()
	manager.handlers[plugin.Name()] = handler
}

func parseSysUeventFile(path string) *usbDevice {
	// Grab all details we are interested from uevent
	file, err := os.Open(filepath.Join(path, "uevent"))
	if err != nil {
		// TODO: high level loggin
		return nil
	}
	defer file.Close()

	u := usbDevice{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		equal := strings.Index(line, "=")
		if strings.HasPrefix(line, "BUSNUM") {
			val, err := strconv.ParseInt(line[equal+1:], 10, 32)
			if err != nil {
				return nil
			}
			u.Bus = int(val)
		} else if strings.HasPrefix(line, "DEVNUM") {
			val, err := strconv.ParseInt(line[equal+1:], 10, 32)
			if err != nil {
				return nil
			}
			u.DeviceNumber = int(val)
		} else if strings.HasPrefix(line, "PRODUCT") {
			values := strings.Split(line[equal+1:], "/")
			if len(values) != 3 {
				return nil
			}

			val, err := strconv.ParseInt(values[0], 16, 32)
			if err != nil {
				return nil
			}
			u.Vendor = int(val)

			val, err = strconv.ParseInt(values[1], 16, 32)
			if err != nil {
				return nil
			}
			u.Product = int(val)

			val, err = strconv.ParseInt(values[2], 16, 32)
			if err != nil {
				return nil
			}
			u.BCD = int(val)
		} else if strings.HasPrefix(line, "DEVNAME") {
			u.Dev = "/dev/" + line[equal+1:]
		}
	}
	return &u
}

func discoverUSBDevices() []*usbDevice {
	usbDevices := make([]*usbDevice, 0)
	err := filepath.Walk("/sys/bus/usb/devices", func(path string, info os.FileInfo, err error) error {
		// Ignore named usb controllers
		if strings.HasPrefix(info.Name(), "usb") {
			return nil
		}
		// We are interested in actual USB devices information that
		// contains idVendor and idProduct. We can skip all others.
		if _, err := os.Stat(filepath.Join(path, "idVendor")); err != nil {
			return nil
		}

		device := parseSysUeventFile(path)
		if device == nil {
			return nil
		}
		usbDevices = append(usbDevices, device)

		// FIXME: Check if device is available ?
		return nil
	})

	if err != nil {
		log.Log.Reason(err).Error("Failed when walking usb devices tree")
	}
	return usbDevices
}
