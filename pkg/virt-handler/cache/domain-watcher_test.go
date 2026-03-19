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

package cache

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	notifyserver "kubevirt.io/kubevirt/pkg/virt-handler/notify-server"
	notifyclient "kubevirt.io/kubevirt/pkg/virt-launcher/notify-client"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
)

type crashableServer struct {
	mu      sync.Mutex
	crashCh chan struct{}
}

func newCrashableServer() *crashableServer {
	return &crashableServer{
		crashCh: make(chan struct{}),
	}
}

func (cs *crashableServer) runServer(virtShareDir string, stopChan chan struct{}, c chan watch.Event, recorder record.EventRecorder, vmiStore cache.Store) error {
	cs.mu.Lock()
	currentCrashCh := cs.crashCh
	cs.mu.Unlock()

	mergedStop := make(chan struct{})
	go func() {
		select {
		case <-stopChan:
			close(mergedStop)
		case <-currentCrashCh:
			close(mergedStop)
		}
	}()
	return notifyserver.RunServer(virtShareDir, mergedStop, c, recorder, vmiStore)
}

func (cs *crashableServer) crash() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	close(cs.crashCh)
	cs.crashCh = make(chan struct{})
}

var _ = Describe("Domain Watcher", func() {
	Context("listSockets ", func() {
		It("should return socket list from ghost record cache", func() {
			const podUID = "5678"
			const socketPath = "/path/to/domainsock"

			ghostCacheDir := GinkgoT().TempDir()

			ghostRecordStore := InitializeGhostRecordCache(NewIterableCheckpointManager(ghostCacheDir))

			err := ghostRecordStore.Add("test-ns", "test-domain", socketPath, podUID)
			Expect(err).ToNot(HaveOccurred())

			socketFiles, err := listSockets(ghostRecordStore.list())
			Expect(err).ToNot(HaveOccurred())
			Expect(socketFiles).To(HaveLen(1))
			Expect(socketFiles[0]).To(Equal(socketPath))

		})
	})

	Context("with notify server", func() {
		var shareDir string
		var stopChan chan struct{}
		var wg *sync.WaitGroup

		BeforeEach(func() {
			var err error
			shareDir, err = os.MkdirTemp("", "kubevirt-share")
			Expect(err).ToNot(HaveOccurred())

			stopChan = make(chan struct{})
			wg = &sync.WaitGroup{}

			notifyServer := filepath.Join(shareDir, "domain-notify.sock")
			pipePath := filepath.Join(shareDir, "domain-notify-pipe.sock")
			Expect(os.Symlink(notifyServer, pipePath)).To(Succeed())
		})

		AfterEach(func() {
			close(stopChan)
			wg.Wait()
			Expect(os.RemoveAll(shareDir)).To(Succeed())
		})

		verifyObj := func(informer cache.SharedInformer, key string, domain *api.Domain, g Gomega) {
			obj, exists, err := informer.GetStore().GetByKey(key)
			g.Expect(err).ToNot(HaveOccurred())

			if domain != nil {
				g.Expect(exists).To(BeTrue())

				eventDomain := obj.(*api.Domain)
				eventDomain.Spec.XMLName = xml.Name{}
				g.Expect(equality.Semantic.DeepEqual(&domain.Spec, &eventDomain.Spec)).To(BeTrue())
			} else {
				g.Expect(exists).To(BeFalse())
			}
		}

		It("should recover from server crash and continue receiving events", func() {
			cs := newCrashableServer()

			d := &domainWatcher{
				backgroundWatcherStarted: false,
				virtShareDir:             shareDir,
				watchdogTimeout:          10,
				unresponsiveSockets:      make(map[string]int64),
				resyncPeriod:             1 * time.Hour,
				runServer:                cs.runServer,
			}

			informer := cache.NewSharedInformer(d, &api.Domain{}, 0)

			wg.Add(1)
			go func() { informer.Run(stopChan); wg.Done() }()
			cache.WaitForCacheSync(stopChan, informer.HasSynced)

			client := notifyclient.NewNotifier(shareDir)
			client.SetCustomTimeouts(500*time.Millisecond, 2*time.Second, 15*time.Second)
			defer client.Close()

			domain := api.NewMinimalDomain("test")

			By("sending an event before crash")
			Expect(client.SendDomainEvent(watch.Event{Type: watch.Added, Object: domain})).To(Succeed())
			Eventually(func(g Gomega) {
				verifyObj(informer, "default/test", domain, g)
			}, 5*time.Second, 200*time.Millisecond).Should(Succeed())

			By("crashing the notify server")
			cs.crash()
			client.Close()

			By("waiting for the server to recover and sending an event after crash")
			updatedDomain := domain.DeepCopy()
			updatedDomain.Spec.UUID = "updated-after-crash"
			Eventually(func() error {
				return client.SendDomainEvent(watch.Event{Type: watch.Modified, Object: updatedDomain})
			}, 15*time.Second, 500*time.Millisecond).Should(Succeed())

			Eventually(func(g Gomega) {
				verifyObj(informer, "default/test", updatedDomain, g)
			}, 5*time.Second, 200*time.Millisecond).Should(Succeed())
		})
	})
})
