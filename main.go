package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/kevinburke/hostsfile/lib"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	hostsFile  = "/etc/hosts"
	serviceBin = "/usr/bin/service"
	glusterBin = "/usr/sbin/gluster"
)

type cmdLabels [][]string

func (self *cmdLabels) String() string {
	return fmt.Sprintf("%#v", self)
}

func (self *cmdLabels) Set(value string) error {
	s := strings.Split(value, "=")
	if len(s) != 2 {
		return errors.New("must be like a \"key=value\"")
	}
	*self = append(*self, s)
	return nil
}

func main() {
	namespaceName := flag.String("namespace", api.NamespaceDefault, "namespace")

	beat := flag.Int64("beat", 5, "beat seconds")

	var selectedLabels cmdLabels
	flag.Var(&selectedLabels, "labels", "--labels key1=value1 --labels key2=value2 ...")

	flag.Parse()

	manager := NewManager(*namespaceName, selectedLabels)

	manager.Run(*beat)
}

type Manager struct {
	Namespace string
	Labels    [][]string

	mutex  *sync.Mutex
	client *client.Client
}

func NewManager(namespace string, labels [][]string) *Manager {
	return &Manager{
		Namespace: namespace,
		Labels:    labels,

		mutex: &sync.Mutex{},
	}
}

func (self *Manager) Run(beat int64) error {
	rpcCmd := self.runRpcService()
	if err := rpcCmd.Wait(); err != nil {
		glog.Errorln(err.Error())
	}

	time.Sleep(time.Second * 2)

	glusterDaemon := self.runGlusterDaemon()

	beatSeconds := time.Duration(beat) * time.Second
	go func(sleep time.Duration) {
		self.joinBeat(sleep)
	}(beatSeconds)

	return glusterDaemon.Wait()
}

func (self *Manager) getClient() (*client.Client, error) {
	return client.NewInCluster()
}

func (self *Manager) createServerCmd() *exec.Cmd {
	cmd := exec.Command(glusterBin, "--log-file=-", "--no-daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (self *Manager) createRpcServiceCmd() *exec.Cmd {
	cmd := exec.Command(serviceBin, "rpcbind", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (self *Manager) runGlusterDaemon() *exec.Cmd {
	serverCmd := self.createServerCmd()
	err := serverCmd.Start()
	if err != nil {
		glog.Fatalln(err.Error())
	}

	return serverCmd
}

func (self *Manager) runRpcService() *exec.Cmd {
	serverCmd := self.createRpcServiceCmd()
	err := serverCmd.Start()
	if err != nil {
		glog.Errorln(err.Error())
	}

	return serverCmd
}

func (self *Manager) getRunningPods() (result []*api.Pod, err error) {
	c, err := self.getClient()
	if err != nil {
		return
	}

	labelSet := labels.Set{}
	for _, v := range self.Labels {
		key := v[0]
		val := v[1]
		labelSet[key] = val
	}

	pods, err := c.Pods(self.Namespace).List(api.ListOptions{LabelSelector: labelSet.AsSelector()})

	for idx, _ := range pods.Items {
		pod := pods.Items[idx]
		podStatus := pod.Status
		if podStatus.Phase == api.PodRunning {
			result = append(result, &pod)
		}
	}
	return
}

func (self *Manager) joinBeat(sleep time.Duration) {
	for range time.Tick(sleep) {
		pods, err := self.getRunningPods()
		if err != nil {
			glog.Errorln(err.Error())
			continue
		}

		for _, pod := range pods {
			if err := self.join(pod); err != nil {
				glog.Errorln(err.Error())
			}
		}
	}
}

func (self *Manager) join(pod *api.Pod) error {
	return self.joinHost(pod)
}

func (self *Manager) joinHost(pod *api.Pod) (err error) {
	podIp := pod.Status.PodIP
	podName := pod.Name

	file, err := os.Open(hostsFile)
	defer file.Close()
	if err != nil {
		return
	}

	h, err := hostsfile.Decode(file)
	if err != nil {
		return
	}
	file.Close()

	ipadrr, err := net.ResolveIPAddr("ip", podIp)
	if err != nil {
		return
	}

	err = h.Set(*ipadrr, podName)
	if err != nil {
		return
	}

	wfile, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_TRUNC, 0644)
	defer wfile.Close()
	if err != nil {
		return
	}

	return hostsfile.Encode(wfile, h)
}
