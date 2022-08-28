package kube

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/main-kube/util/safe"
	"github.com/main-kube/util/slice"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// pod map key=namespace
	Map = safe.NewSortedMap(map[string]*safe.SortedMap[string, *Pod]{}, func(data []string, i, j int) bool {
		return data[i] < data[j]
	})
)

type Pod struct {
	*PodPortForwardA
	Name      string
	Namespace string
	Status    string
	Ports     []string
}
type MapUpdateMsg struct{}

func newPodMap() *safe.SortedMap[string, *Pod] {
	return safe.NewSortedMap(map[string]*Pod{}, func(data []string, i, j int) bool {
		return data[i] < data[j]
	})
}

func UpdateMap(notify chan any) {
	wg := sync.WaitGroup{}
	for range time.Tick(2 * time.Second) {
		if Client == nil {
			continue
		}
		ns, _ := Client.API.CoreV1().Namespaces().List(Client.CTX, v1.ListOptions{})
		// delete nonexistent namespaces
		go cleanMap(ns.Items)
		for _, namespace := range ns.Items {
			wg.Add(1)
			go func(namespace string) {
				defer wg.Done()
				go fixPods(namespace, notify)
			}(namespace.Name)
		}
		wg.Wait()
		// TODO notify front
	}
}

func cleanMap(ns []corev1.Namespace) {
	if Map.Len() > 0 {
		var nsNameList []string
		for _, n := range ns {
			nsNameList = append(nsNameList, n.Name)
		}
		for _, key := range slice.Diff(Map.Keys(), nsNameList) {
			Map.Delete(key)
		}
	}
}

// DON'T LOOK
func fixPods(nsName string, notify chan any) {
	podlist, err := Client.API.CoreV1().Pods(nsName).List(Client.CTX, v1.ListOptions{})
	if err != nil {
		log.Error(err)
	}
	podMap, ok := Map.GetFull(nsName)
	if !ok {
		podMap = newPodMap()
	}
	nameList := make([]string, 0, len(podlist.Items))
	for _, p := range podlist.Items {
		nameList = append(nameList, p.Name)
		pod, ok := podMap.GetFull(p.Name)
		if ok {
			if string(p.Status.Phase) != pod.Status {
				pod.Status = string(p.Status.Phase)
				podMap.Set(p.Name, pod)
			}
			continue
		}
		podMap.Set(p.Name, &Pod{
			Name:      p.Name,
			Namespace: nsName,
			Status:    string(p.Status.Phase),
			Ports:     fillPorts(p),
		})
	}
	for _, element := range slice.Diff(nameList, podMap.Keys()) {
		podMap.Delete(element)
	}

	Map.Set(nsName, podMap)
	notify <- MapUpdateMsg{}
}

func fillPorts(p corev1.Pod) (ports []string) {
	for _, c := range p.Spec.Containers {
		for _, port := range c.Ports {
			ports = append(ports, strconv.Itoa(int(port.ContainerPort)))
		}
	}
	return
}

func (p *Pod) Ping() {
	log.Info("------------------------- dupa -------------------------")
	if p.PodPortForwardA == nil {
		log.Info("nil ", p.Name)
		return
	}
	log.Info("ping ", p.Name)
	_, err := http.Get(fmt.Sprintf("localhost:%d", p.LocalPort))
	if err != nil {
		log.Info(err)
		p.Condition = false
		return
	}
	p.Condition = true
}