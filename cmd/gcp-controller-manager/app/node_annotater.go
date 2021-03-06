/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	core "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/golang/glog"
	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v1"
)

const InstanceIDAnnotationKey = "container.googleapis.com/instance_id"

type nodeAnnotater struct {
	c           clientset.Interface
	ns          corelisters.NodeLister
	hasSynced   func() bool
	queue       workqueue.RateLimitingInterface
	getInstance func(project, zone, instance string) (*compute.Instance, error)
}

func newNodeAnnotater(client clientset.Interface, nodeInformer coreinformers.NodeInformer, gts oauth2.TokenSource) (*nodeAnnotater, error) {

	oclient := oauth2.NewClient(context.Background(), gts)
	cs, err := compute.New(oclient)
	if err != nil {
		return nil, fmt.Errorf("creating GCE API client: %v", err)
	}
	gce := compute.NewInstancesService(cs)

	annotater := &nodeAnnotater{
		c:         client,
		ns:        nodeInformer.Lister(),
		hasSynced: nodeInformer.Informer().HasSynced,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), "node-annotater"),
		getInstance: func(project, zone, instance string) (*compute.Instance, error) {
			return gce.Get(project, zone, instance).Do()
		},
	}
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    annotater.add,
		UpdateFunc: annotater.update,
	})
	return annotater, nil
}

func (na *nodeAnnotater) add(obj interface{}) {
	na.enqueue(obj)
}

func (na *nodeAnnotater) update(obj, oldObj interface{}) {
	node := obj.(*core.Node)
	oldNode := oldObj.(*core.Node)
	if node.Status.NodeInfo.BootID != oldNode.Status.NodeInfo.BootID {
		na.enqueue(obj)
	}
}

func (na *nodeAnnotater) enqueue(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}
	na.queue.Add(key)
}

func (na *nodeAnnotater) Run(workers int, stopCh <-chan struct{}) {
	if !controller.WaitForCacheSync("node-annotater", stopCh, na.hasSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(na.work, time.Second, stopCh)
	}
	<-stopCh
}

func (na *nodeAnnotater) processNextWorkItem() bool {
	key, quit := na.queue.Get()
	if quit {
		return false
	}
	defer na.queue.Done(key)

	na.sync(key.(string))
	na.queue.Forget(key)

	return true
}

func (na *nodeAnnotater) work() {
	for na.processNextWorkItem() {
	}
}

func (na *nodeAnnotater) sync(key string) {
	node, err := na.ns.Get(key)
	if err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}

	eid, err := na.getExternalID(node.Spec.ProviderID)
	if err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}
	if len(node.ObjectMeta.Annotations) != 0 && eid == node.ObjectMeta.Annotations[InstanceIDAnnotationKey] {
		// node restarted but no update of ExternalID required
		return
	}
	if node.ObjectMeta.Annotations == nil {
		node.ObjectMeta.Annotations = make(map[string]string)
	}

	node.ObjectMeta.Annotations[InstanceIDAnnotationKey] = eid

	if _, err := na.c.Core().Nodes().Update(node); err != nil {
		glog.Errorf("Sync %v failed with: %v", key, err)
		na.queue.Add(key)
		return
	}
}

func (na *nodeAnnotater) getExternalID(nodeUrl string) (string, error) {
	u, err := url.Parse(nodeUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse %q: %v", nodeUrl, err)
	}
	if u.Scheme != "gce" {
		return "", fmt.Errorf("instance %q doesn't run on gce", nodeUrl)
	}
	project := u.Host
	parts := strings.Split(u.Path, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("failed to parse %q: expected a three part path")
	}
	if len(parts[0]) != 0 {
		return "", fmt.Errorf("failed to parse %q: part one of path to have length 0")
	}
	zone := parts[1]
	vm := parts[2]

	instance, err := na.getInstance(project, zone, vm)
	if err != nil {
		return "", fmt.Errorf("unable to query gcp apis: %v", err)
	}

	return strconv.FormatUint(instance.Id, 10), nil
}
