package pod_lister

import (
	"fmt"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/container_lister"
)

type PodmanList struct {
}

func (k *PodmanList) GetSystemProcessName() string {
	return systemProcessName
}

//1
func (k *PodmanList) GetPodNameFromcGgroupID(cGroupID uint64) (string, error) {

	info, err := container_lister.GetContainerNameFromcGgroupID(cGroupID)
	if err != nil {
		return "", err
	}
	if info == "unknown" {
		return "systemProcess", nil
	}
	return info, nil

}

func (k *PodmanList) GetPodNameSpaceFromcGgroupID(cGroupID uint64) (string, error) {
	return "", nil
}

func (k *PodmanList) GetPodContainerNameFromcGgroupID(cGroupID uint64) (string, error) {

	info, err := container_lister.GetContainerNameFromcGgroupID(cGroupID)
	return info, err

}

func (k *PodmanList) ReadAllCgroupIOStat() (uint64, uint64, int, error) {

	return container_lister.ReadIOStat(cgroupPath)

}

func (k *PodmanList) ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error) {
	path, err := container_lister.GetPathFromcGroupID(cGroupID)
	if err != nil {
		return 0, 0, 0, err
	}
	if strings.Contains(path, "libpod-") {
		return container_lister.ReadIOStat(cgroupPath + path)
	}
	return 0, 0, 0, fmt.Errorf("no cgroup path found")

}

func (k *PodmanList) GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {

	return nil, nil, 0, 0, nil

}
