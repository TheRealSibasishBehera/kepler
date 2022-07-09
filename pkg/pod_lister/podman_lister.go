package pod_lister

type PodmanList struct {
}

// All these functions are the ones that need to be edited to be adapted to podman
func (k *PodmanList) GetSystemProcessName() string {
	return systemProcessName
}

// func (k *PodmanList) GetPodNameFromcGgroupID(cGroupID uint64) (string, error) {
// }

// func (k *PodmanList) GetPodNameSpaceFromcGgroupID(cGroupID uint64) (string, error) {
// }

// func (k *PodmanList) GetPodContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
// }

// func (k *PodmanList) ReadAllCgroupIOStat() (uint64, uint64, int, error) {
// }

// func (k *PodmanList) ReadCgroupIOStat(cGroupID uint64) (uint64, uint64, int, error) {
// }

// func (k *PodmanList) GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {
// }
