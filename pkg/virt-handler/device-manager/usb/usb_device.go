package usb

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"kubevirt.io/client-go/log"
	device_manager "kubevirt.io/kubevirt/pkg/virt-handler/device-manager"
	devicepluginapi "kubevirt.io/kubevirt/pkg/virt-handler/device-manager/deviceplugin/v1beta1"
)

type usb struct{}

type usbDevicePlugin struct {
	socketPath   string
	stop         <-chan struct{}
	server       *grpc.Server
	resourceName string
	devices      []*devicepluginapi.Device
}

var _ devicepluginapi.DevicePluginServer = &usbDevicePlugin{}

func (plugin *usbDevicePlugin) Start(stop <-chan struct{}) error {
	plugin.stop = stop

	sock, err := net.Listen("unix", plugin.socketPath)
	if err != nil {
		return fmt.Errorf("error creating GRPC server socket: %v", err)
	}

	plugin.server = grpc.NewServer([]grpc.ServerOption{}...)

	devicepluginapi.RegisterDevicePluginServer(plugin.server, plugin)

	errChan := make(chan error, 2)

	go func() {
		errChan <- plugin.server.Serve(sock)
	}()

	err = device_manager.WaitForGRPCServer(plugin.socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("error starting the GRPC server: %v", err)
	}

	err = plugin.register()
	if err != nil {
		return fmt.Errorf("error registering with device plugin manager: %v", err)
	}

	log.Log.Infof("%s device plugin started", plugin.resourceName)
	err = <-errChan
	return err
}

func (plugin *usbDevicePlugin) register() error {
	return nil
}

func (plugin *usbDevicePlugin) GetDevicePluginOptions(ctx context.Context, _ *devicepluginapi.Empty) (*devicepluginapi.DevicePluginOptions, error) {
	return &devicepluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

func (plugin *usbDevicePlugin) ListAndWatch(_ *devicepluginapi.Empty, lws devicepluginapi.DevicePlugin_ListAndWatchServer) error {
	// TODO Send devices
	lws.Send(&devicepluginapi.ListAndWatchResponse{Devices: plugin.devices})

loop:
	for {
		select {
		// TODO add a health check, e.g usb was unplugged
		case <-plugin.stop:
			break loop
		}
	}
	if err := lws.Send(&devicepluginapi.ListAndWatchResponse{Devices: []*devicepluginapi.Device{}}); err != nil {
		log.Log.Reason(err).Warningf("Failed to deregister device plugin %s", plugin.resourceName)
	}
	return nil
}

func (plugin *usbDevicePlugin) Allocate(_ context.Context, allocRequest *devicepluginapi.AllocateRequest) (*devicepluginapi.AllocateResponse, error) {
	resposne := new(devicepluginapi.AllocateResponse)
	for _, request := range allocRequest.ContainerRequests {
		for _, _ = range request.DevicesIDs {
			// collect devices
		}
		// TODO
		containerResponse := &devicepluginapi.ContainerAllocateResponse{
			Envs:        nil,
			Mounts:      nil,
			Devices:     nil,
			Annotations: nil,
		}
		resposne.ContainerResponses = append(resposne.ContainerResponses, containerResponse)

	}

	return resposne, nil
}

func (plugin *usbDevicePlugin) PreStartContainer(context.Context, *devicepluginapi.PreStartContainerRequest) (*devicepluginapi.PreStartContainerResponse, error) {
	return &devicepluginapi.PreStartContainerResponse{}, nil
}

type Plugin interface {
	Start(stop <-chan struct{}) (err error)
}

func NewUSBDevicePlugin(resourceName string, usbs []usb) Plugin {
	serverSock := device_manager.SocketPath(resourceName)
	// convert usbs to devices
	devices := []*devicepluginapi.Device{}
	for _, usb := range usbs {
		devices = append(devices, toDevice(usb))
	}

	return &usbDevicePlugin{
		socketPath:   serverSock,
		resourceName: resourceName,
		devices:      devices,
	}
}

func toDevice(device usb) *devicepluginapi.Device {
	return &devicepluginapi.Device{
		ID:       "<todo>",
		Health:   devicepluginapi.Healthy,
		Topology: nil,
	}
}
