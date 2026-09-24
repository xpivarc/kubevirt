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
 * Copyright The KubeVirt Authors.
 *
 */

package launcher_clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"golang.org/x/sync/singleflight"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/record"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	virtcache "kubevirt.io/kubevirt/pkg/virt-handler/cache"
	cmdclient "kubevirt.io/kubevirt/pkg/virt-handler/cmd-client"
	"kubevirt.io/kubevirt/pkg/virt-handler/isolation"
	"kubevirt.io/kubevirt/pkg/virt-handler/notify-server/pipe"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/pkg/vmitrait"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmdv1 "kubevirt.io/kubevirt/pkg/handler-launcher-com/cmd/v1"
)

type LauncherClientsManager interface {
	GetVerifiedLauncherClient(vmi *v1.VirtualMachineInstance) (client cmdclient.LauncherClient, err error)
	GetLauncherClient(vmi *v1.VirtualMachineInstance) (cmdclient.LauncherClient, error)
	GetLauncherClientInfo(vmi *v1.VirtualMachineInstance) *virtcache.LauncherClientInfo
	CloseLauncherClient(vmi *v1.VirtualMachineInstance)
	IsLauncherClientUnresponsive(vmi *v1.VirtualMachineInstance) (unresponsive bool, initialized bool, err error)
}

type launcherClientsManager struct {
	virtShareDir         string
	connGroup            singleflight.Group
	launcherClients      virtcache.LauncherClientInfoByVMI
	podIsolationDetector isolation.PodIsolationDetector
	directChan           chan<- watch.Event
	recorder             record.EventRecorder
}

func NewLauncherClientsManager(
	virtShareDir string,
	podIsolationDetector isolation.PodIsolationDetector,
	directChan chan<- watch.Event,
) LauncherClientsManager {

	l := &launcherClientsManager{
		virtShareDir:         virtShareDir,
		launcherClients:      virtcache.LauncherClientInfoByVMI{},
		podIsolationDetector: podIsolationDetector,
		directChan:           directChan,
	}

	return l
}

func (l *launcherClientsManager) GetVerifiedLauncherClient(vmi *v1.VirtualMachineInstance) (client cmdclient.LauncherClient, err error) {
	client, err = l.GetLauncherClient(vmi)
	if err != nil {
		return
	}

	// Verify connectivity.
	// It's possible the pod has already been torn down along with the VirtualMachineInstance.
	err = client.Ping()
	return
}

func (l *launcherClientsManager) GetLauncherClient(vmi *v1.VirtualMachineInstance) (cmdclient.LauncherClient, error) {
	// Fast path: return cached connection without any synchronization.
	clientInfo, exists := l.launcherClients.Load(vmi.UID)
	if exists && clientInfo.Client != nil {
		return clientInfo.Client, nil
	}

	// Slow path: use singleflight to ensure only one connection is created per VMI
	// even when multiple controllers (VM, MigrationSource, MigrationTarget) race
	// on the same VMI concurrently. Other VMIs are not blocked.
	result, err, _ := l.connGroup.Do(string(vmi.UID), func() (any, error) {
		// Re-check: another goroutine may have created the connection while we waited.
		if clientInfo, exists := l.launcherClients.Load(vmi.UID); exists && clientInfo.Client != nil {
			return clientInfo.Client, nil
		}

		socketFile, err := cmdclient.FindSocket(vmi)
		if err != nil {
			return nil, err
		}

		err = virtcache.GhostRecordGlobalStore.Add(vmi.Namespace, vmi.Name, socketFile, vmi.UID)
		if err != nil {
			return nil, err
		}

		client, err := cmdclient.NewClient(socketFile)
		if err != nil {
			return nil, err
		}

		domainNotifyStopChan := make(chan struct{})
		err = l.startDomainNotify(domainNotifyStopChan, vmi, client)
		if err != nil {
			client.Close()
			close(domainNotifyStopChan)
			return nil, err
		}

		l.launcherClients.Store(vmi.UID, &virtcache.LauncherClientInfo{
			Client:              client,
			SocketFile:          socketFile,
			DomainPipeStopChan:  domainNotifyStopChan,
			NotInitializedSince: time.Now(),
			Ready:               true,
		})

		return client, nil
	})
	if err != nil {
		return nil, err
	}

	return result.(cmdclient.LauncherClient), nil
}

func (l *launcherClientsManager) GetLauncherClientInfo(vmi *v1.VirtualMachineInstance) *virtcache.LauncherClientInfo {
	launcherInfo, exists := l.launcherClients.Load(vmi.UID)
	if !exists {
		return nil
	}
	return launcherInfo
}

func (l *launcherClientsManager) CloseLauncherClient(vmi *v1.VirtualMachineInstance) {
	// UID is required in order to close socket
	if string(vmi.GetUID()) == "" {
		return
	}

	clientInfo, exists := l.launcherClients.Load(vmi.UID)
	if exists {
		clientInfo.Close()
	}

	virtcache.GhostRecordGlobalStore.Delete(vmi.Namespace, vmi.Name)
	l.launcherClients.Delete(vmi.UID)
}

func (l *launcherClientsManager) IsLauncherClientUnresponsive(vmi *v1.VirtualMachineInstance) (unresponsive bool, initialized bool, err error) {
	var socketFile string

	clientInfo, exists := l.launcherClients.Load(vmi.UID)
	if exists {
		if clientInfo.Ready {
			// use cached socket if we previously established a connection
			socketFile = clientInfo.SocketFile
		} else {
			socketFile, err = cmdclient.FindSocket(vmi)
			if err != nil {
				// socket does not exist, but let's see if the pod is still there
				if _, err = cmdclient.FindPodDirOnHost(vmi, cmdclient.SocketDirectoryOnHost); err != nil {
					// no pod meanst that waiting for it to initialize makes no sense
					return true, true, nil
				}
				// pod is still there, if there is no socket let's wait for it to become ready
				if clientInfo.NotInitializedSince.Before(time.Now().Add(-3 * time.Minute)) {
					return true, true, nil
				}
				return false, false, nil
			}
			clientInfo.Ready = true
			clientInfo.SocketFile = socketFile
		}
	} else {
		clientInfo := &virtcache.LauncherClientInfo{
			NotInitializedSince: time.Now(),
			Ready:               false,
		}
		l.launcherClients.Store(vmi.UID, clientInfo)
		// attempt to find the socket if the established connection doesn't currently exist.
		socketFile, err = cmdclient.FindSocket(vmi)
		// no socket file, no VMI, so it's unresponsive
		if err != nil {
			// socket does not exist, but let's see if the pod is still there
			if _, err = cmdclient.FindPodDirOnHost(vmi, cmdclient.SocketDirectoryOnHost); err != nil {
				// no pod means that waiting for it to initialize makes no sense
				return true, true, nil
			}
			return false, false, nil
		}
		clientInfo.Ready = true
		clientInfo.SocketFile = socketFile
	}
	return cmdclient.IsSocketUnresponsive(socketFile), true, nil
}

func handleDomainNotifyPipe(ctx context.Context, ln net.Listener, virtShareDir string, vmi *v1.VirtualMachineInstance) {
	logger := log.Log.Object(vmi)
	fdChan := pipe.ChanFromListener(ctx, logger, ln)

	// Process new connections
	// exit when stop encountered
	go pipe.Pipe(ctx, fdChan, func(conn net.Conn) {
		pipe.Proxy(logger, conn, pipe.NewConnectToNotifyFunc(virtShareDir))
	})
}

func processDomainEvents(stream cmdclient.DomainEventsStream, directChan chan<- watch.Event) {
	for {
		response, err := stream.Recv()
		if err != nil {
			// log
			return
		}

		switch t := response.Type.(type) {
		case *cmdv1.DomainEventsResponse_DomainJSON:
			domain := &api.Domain{}
			err := json.Unmarshal(t.DomainJSON, domain)
			if err != nil {
				log.Log.Errorf("Failed to unmarshal domain json object")
			}
			log.Log.Object(domain).V(3).Infof("Received Domain Event of type %s", response.EventType)
			switch response.EventType {
			case string(watch.Added):
				directChan <- watch.Event{Type: watch.Added, Object: domain}
			case string(watch.Modified):
				directChan <- watch.Event{Type: watch.Modified, Object: domain}
			case string(watch.Deleted):
				directChan <- watch.Event{Type: watch.Deleted, Object: domain}
			case string(watch.Error):
				// log.Log.Object(domain).Errorf("Domain error event with message: %s", status.Message)
			}
		case *cmdv1.DomainEventsResponse_StatusJSON:
			status := &metav1.Status{}
			err := json.Unmarshal(t.StatusJSON, status)
			if err != nil {
				log.Log.Errorf("Failed to unmarshal status json object")
			}
			log.Log.V(3).Infof("Received Domain Event of type %s", response.EventType)
			switch response.EventType {
			case string(watch.Added):
			case string(watch.Modified):
			case string(watch.Deleted):
				// TODO
			case string(watch.Error):
				log.Log.Errorf("Domain error event with message: %s", status.Message)
			}
		}
	}
}

func processKubernetesEvents(stream cmdclient.KubernetesEventsStream, recorder record.EventRecorder, vmi *v1.VirtualMachineInstance) {
	for {
		response, err := stream.Recv()
		if err != nil {
			// log
			return
		}

		// unmarshal k8s event
		var event k8sv1.Event
		err = json.Unmarshal(response.EventJSON, &event)
		if err != nil {
			// log
			continue
		}
		recorder.Event(vmi, event.Type, event.Reason, event.Message)
	}
}

func (l *launcherClientsManager) startDomainNotify(domainPipeStopChan <-chan struct{}, vmi *v1.VirtualMachineInstance,
	client cmdclient.LauncherClient) error {

	// TODO need to use one ctx
	ctx := contextFromChan(domainPipeStopChan)
	stream, err := client.DomainEvents(ctx)
	if err != nil {
		if cmdclient.IsUnimplemented(err) {
			return l.startDomainNotifyPipe(domainPipeStopChan, vmi)
		}
		return err
	}

	go func(stream cmdclient.DomainEventsStream) {
		for {
			processDomainEvents(stream, l.directChan)
			select {
			case <-ctx.Done():
				return
			default:
				newStream, err := client.DomainEvents(ctx)
				if err != nil {
					continue
				}
				stream = newStream
			}
		}
	}(stream)

	go func() {
		for {
			stream, err := client.KubernetesEvents(ctx)
			if err != nil {
				continue
			}
			processKubernetesEvents(stream, l.recorder, vmi)
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return nil
}

func (l *launcherClientsManager) startDomainNotifyPipe(domainPipeStopChan <-chan struct{}, vmi *v1.VirtualMachineInstance) error {

	res, err := l.podIsolationDetector.Detect(vmi)
	if err != nil {
		return fmt.Errorf("failed to detect isolation for launcher pod when setting up notify pipe: %v", err)
	}

	listener, err := pipe.InjectNotify(res, l.virtShareDir, vmitrait.IsNonRoot(vmi))
	if err != nil {
		return err
	}
	ctx := contextFromChan(domainPipeStopChan)
	handleDomainNotifyPipe(ctx, listener, l.virtShareDir, vmi)

	return nil
}

func contextFromChan(c <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		cancel()
	}()
	return ctx
}
