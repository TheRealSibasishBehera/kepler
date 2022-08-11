package container_lister

import (
	procfs "github.com/prometheus/procfs"
)

var mem_fs, _ = procfs.NewFS("/proc/meminfo")

func GetEdgeDeviceMemInfo() (*procfs.Meminfo, error) {

	edm, err := mem_fs.Meminfo()
	if err != nil {
		return &procfs.Meminfo{}, err
	}
	return &edm, nil
}

//getting info from /proc/meminfo
func GetEdgeDeviceMemInBytes() (uint64, error) {
	x, err := GetEdgeDeviceMemInfo()
	if err != nil {
		return 0, err
	}
	TotalAvailable := x.MemTotal
	TotalFree := x.MemFree
	return *TotalAvailable - *TotalFree, nil

}
