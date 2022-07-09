package pod_lister

import (
	"fmt"
	"strings"
)

type PodLister interface {
	GetSystemProcessName() string
	GetPodNameFromcGgroupID(cGroupID uint64) (string, error)
	GetPodNameSpaceFromcGgroupID(cGroupID uint64) (string, error)
	GetPodContainerNameFromcGgroupID(cGroupID uint64) (string, error)
	ReadAllCgroupIOStat() (uint64, uint64, int, error)
	ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error)
	GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error)
}

type KubList struct {
}

func (k *KubList) GetSystemProcessName() string {
	return systemProcessName
}

func (k *KubList) GetPodNameFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.PodName, err
}

func (k *KubList) GetPodNameSpaceFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.Namespace, err
}

func (k *KubList) GetPodContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.ContainerName, err
}

func (k *KubList) ReadAllCgroupIOStat() (uint64, uint64, int, error) {
	return readIOStat(cgroupPath)
}

func (k *KubList) ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error) {
	path, err := getPathFromcGroupID(cGroupID)
	if err != nil {
		return 0, 0, 0, err
	}
	if strings.Contains(path, "crio-") {
		return readIOStat(path)
	}
	return 0, 0, 0, fmt.Errorf("no cgroup path found")
}

func (k *KubList) GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {
	return podLister.ListMetrics()
}
