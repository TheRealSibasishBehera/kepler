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

func (k *PodmanList) GetPodNameFromcGgroupID(cGroupID uint64) (string, error) {

	info, err := container_lister.GetContainerNameFromcGgroupID(cGroupID)
	if err != nil {
		return "", err
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

	//works same for for both pod and containers
	return readIOStat(cgroupPath)

}

func (k *PodmanList) ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error) {
	path, err := container_lister.GetPathFromcGroupID(cGroupID)
	if err != nil {
		return 0, 0, 0, err
	}
	if strings.Contains(path, "libpod-") {
		return readIOStat(path)
	}
	return 0, 0, 0, fmt.Errorf("no cgroup path found")

}

func (k *PodmanList) GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {

	return nil, nil, 0, 0, nil

}
