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
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	hostsFile = "/etc/hosts"
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
	serviceName := flag.String("service", "glusterfs-storage", "service name")

	replicaCount := flag.Int64("replica", 1, "replica count")
	beat := flag.Int64("beat", 5, "beat seconds")
	volumeName := flag.String("volume", "media", "volume name")

	var selectedLabels cmdLabels
	flag.Var(&selectedLabels, "labels", "--labels key1=value1 --labels key2=value2 ...")

	flag.Parse()

	manager := NewVolumeManager(*volumeName, *namespaceName, *serviceName, *replicaCount, selectedLabels)

	manager.Run(*beat)
}

type PeerInfo struct {
	Hostname string
	Uuid     string
	State    string
}

type VolumeManager struct {
	Name      string
	Replica   int64
	Namespace string
	Service   string
	Labels    [][]string

	mutex  *sync.Mutex
	client *client.Client
}

func NewVolumeManager(name, namespace, service string, replica int64, labels [][]string) *VolumeManager {
	return &VolumeManager{
		Name:      name,
		Replica:   replica,
		Namespace: namespace,
		Service:   service,
		Labels:    labels,

		mutex: &sync.Mutex{},
	}
}

func (self *VolumeManager) Run(beat int64) error {
	rpcCmd := self.runRpcService()
	rpcCmd.Wait()

	time.Sleep(time.Second * 2)

	glusterDaemon := self.runGlusterDaemon()

	beatSeconds := time.Duration(beat) * time.Second
	go func(sleep time.Duration) {
		self.joinBeat(sleep)
	}(beatSeconds)

	return glusterDaemon.Wait()
}

func (self *VolumeManager) getClient() *client.Client {
	if self.client == nil {
		self.mutex.Lock()

		if self.client != nil {
			return self.client
		}

		if c, err := client.NewInCluster(); err != nil {
			glog.Fatalln("Can't connect to Kubernetes API:", err)
		} else {
			self.client = c
		}
		self.mutex.Unlock()
	}
	return self.client
}

func (self *VolumeManager) createServerCmd() *exec.Cmd {
	cmd := exec.Command("/usr/sbin/glusterd", "--log-file=-", "--no-daemon")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (self *VolumeManager) createRpcServiceCmd() *exec.Cmd {
	cmd := exec.Command("/usr/bin/service", "rpcbind", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (self *VolumeManager) runGlusterDaemon() *exec.Cmd {
	serverCmd := self.createServerCmd()
	err := serverCmd.Start()
	if err != nil {
		glog.Fatal(err)
	}

	return serverCmd
}

func (self *VolumeManager) runRpcService() *exec.Cmd {
	serverCmd := self.createRpcServiceCmd()
	err := serverCmd.Start()
	if err != nil {
		glog.Fatal(err)
	}

	return serverCmd
}

func (self *VolumeManager) getRunningPods() []*api.Pod {
	c := self.getClient()

	labelSet := labels.Set{}
	for _, v := range self.Labels {
		key := v[0]
		val := v[1]
		labelSet[key] = val
	}

	pods, _ := c.Pods(self.Namespace).List(api.ListOptions{LabelSelector: labelSet.AsSelector()})

	var result []*api.Pod
	for idx, _ := range pods.Items {
		pod := pods.Items[idx]
		podStatus := pod.Status
		if podStatus.Phase == api.PodRunning {
			result = append(result, &pod)
		}
	}
	return result
}

func (self *VolumeManager) joinBeat(sleep time.Duration) {
	for range time.Tick(sleep) {
		pods := self.getRunningPods()
		for _, pod := range pods {
			if err := self.join(pod); err != nil {
				glog.Warningln(err.Error())
			}
		}
	}
}

func (self *VolumeManager) join(pod *api.Pod) error {
	return self.joinHost(pod)
}

func (self *VolumeManager) attache(pod *api.Pod) {
	podIpd := pod.Status.PodIP
	output, err := self.runGlusterCommand("peer", "probe", podIpd)
	if err != nil {
		os.Stderr.Write(output)
	}
}

func (self *VolumeManager) joinHost(pod *api.Pod) error {
	podIp := pod.Status.PodIP
	podName := pod.Name

	if file, err := os.Open(hostsFile); err != nil {
		return err
	} else {
		defer file.Close()
		if h, err := hostsfile.Decode(file); err != nil {
			return err
		} else {
			file.Close()
			if ipadrr, err := net.ResolveIPAddr("ip", podIp); err != nil {
				return err
			} else {
				if err := h.Set(*ipadrr, podName); err != nil {
					return err
				} else {
					if wfile, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_TRUNC, 0644); err != nil {
						return err
					} else {
						defer wfile.Close()
						if err = hostsfile.Encode(wfile, h); err != nil {
							return err
						}
					}

				}

			}
		}
	}
	return nil
}

func (self *VolumeManager) getAllPeers() map[string]PeerInfo {
	output, _ := self.runGlusterCommand("peer", "status")
	re := regexp.MustCompile(`((?:Hostname):(.+)\n(?:Uuid):(.+)\n(?:State):(.+))`)
	res := re.FindAllSubmatch(output, -1)
	result := make(map[string]PeerInfo, len(res))
	for _, v := range res {
		hostname := strings.TrimSpace(string(v[1]))
		result[hostname] = PeerInfo{
			Hostname: hostname,
			Uuid:     strings.TrimSpace(string(v[2])),
			State:    strings.TrimSpace(string(v[3])),
		}
	}
	return result
}

func (self *VolumeManager) runGlusterCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("/usr/sbin/gluster", args...)
	return cmd.CombinedOutput()
}
