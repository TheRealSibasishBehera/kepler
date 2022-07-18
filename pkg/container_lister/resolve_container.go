/*
Copyright 2021.

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

package container_lister

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/containers/podman/v3/pkg/bindings/containers"

	"golang.org/x/sys/unix"

	bpf "github.com/iovisor/gobpf/bcc"
)

type ContainerInfo struct {
	ContainerName string
	ContainerID   string
}

const (
	systemProcessName      string = "system_processes"
	systemProcessNamespace string = "system"
	// containerIDPredix      string = "cri-o://"
	unknownPath string = "unknown"
)

var (
	containerLister            PodmanContainerLister
	cGroupIDToContainerIDCache = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}

	cGroupIDToPath = map[uint64]string{}

	// containerID
	containerIDToCgroup_id = map[string]uint64{}
	re                     = regexp.MustCompile(`libpod-(.*?)\.scope`)
	cgroupPath             = "/sys/fs/cgroup"
	byteOrder              binary.ByteOrder
	ctx                    *context.Context
)

func init() {
	byteOrder = bpf.GetHostByteOrder()
	containerLister = PodmanContainerLister{}
	updateListContainerCache("", false)

	//start the podman socket
	ctx := StartingPodmanSocket()
}

func GetSystemProcessName() string {
	return systemProcessName
}

func GetContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.ContainerName, err
}

func GetContainerIDFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.ContainerID, err
}

func getContainerInfoFromcGgroupID(cGroupID uint64) (*ContainerInfo, error) {
	var err error
	var containerID string
	info := &ContainerInfo{}

	if containerID, err = getContainerIDFromcGroupID(cGroupID); err != nil {
		//TODO: print a warn with high verbosity
		return info, nil
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	// update cache info and stop loop if container id found
	updateListContainerCache(containerID, true)
	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	containerIDToContainerInfo[containerID] = info
	return containerIDToContainerInfo[containerID], nil
}

// updateListPodCache updates cache info with all pods and optionally
// stops the loop when a given container ID is found
func updateListContainerCache(targetContainerID string, stopWhenFound bool) {

	//get all the containers
	containers, err := containerLister.ListContainers()
	if err != nil {
		log.Fatal(err)
	}

	//range over the containers
	for _, container := range containers {
		info := &ContainerInfo{
			ContainerName: container.Names[0],
			ContainerID:   container.ID,
		}

		// containerID := strings.Trim(container.ID, containerIDPredix)
		containerID := info.ContainerID

		Cgroup_path := GetCGroupPathFromContainerID(containerID) // usind default method
		Cgroup_id, err := GetCGroupIDFromCGroupPath(Cgroup_path)

		if err != nil {
			return
		}
		containerIDToCgroup_id[containerID] = Cgroup_id
		cGroupIDToContainerIDCache[Cgroup_id] = containerID

		containerIDToContainerInfo[containerID] = info
		if stopWhenFound && container.ID == targetContainerID {
			return
		}
	}

}

func getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	if id, ok := cGroupIDToContainerIDCache[cGroupID]; ok {
		return id, nil
	}

	//added another search using
	p, err1 := GetContainerIDFromcGgroupID(cGroupID)
	if err1 != nil {
		log.Fatal(err1)
	} else {
		return p, nil
	}

	var err error
	var path string
	if path, err = GetPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	if re.MatchString(path) {
		sub := re.FindAllString(path, -1)
		for _, element := range sub {
			// if strings.Contains(element, "-conmon-") || strings.Contains(element, ".service") {
			// 	return "", fmt.Errorf("process cGroupID %d is not in a kubernetes pod", cGroupID)
			// } else
			if strings.Contains(element, "libpod") {
				containerID := strings.Trim(element, "libpod-")
				containerID = strings.Trim(containerID, ".scope")
				cGroupIDToContainerIDCache[cGroupID] = containerID
				return cGroupIDToContainerIDCache[cGroupID], nil
			}
		}
	}

	cGroupIDToContainerIDCache[cGroupID] = systemProcessName
	return cGroupIDToContainerIDCache[cGroupID], fmt.Errorf("failed to find container with cGroup id: %v", cGroupID)
}

// GetPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func GetPathFromcGroupID(cgroupId uint64) (string, error) {

	// // METHOD 2
	// GET CONTAINER ID FROM CGROUP ID
	// // USE ID to get path{note that this only applicable if the purpose is to find path of container}

	//check if cgroup if has a container associated , if ok then get path
	if p, ok := cGroupIDToContainerIDCache[cgroupId]; ok {
		return GetCGroupPathFromContainerID(p), nil
	}

	//if not found try to update cache
	container_info, err1 := getContainerInfoFromcGgroupID(cgroupId)
	if err1 != nil {
		fmt.Errorf("%s", err1)
	}
	container_id := container_info.ContainerID
	updateListContainerCache(container_id, true)

	// check again
	if p, ok := cGroupIDToContainerIDCache[cgroupId]; ok {
		return GetCGroupPathFromContainerID(p), nil
	}

	if p, ok := cGroupIDToPath[cgroupId]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPath, func(path string, dentry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !dentry.IsDir() {
			return nil
		}
		handle, _, err := unix.NameToHandleAt(unix.AT_FDCWD, path, 0)
		if err != nil {
			return fmt.Errorf("error resolving handle: %v", err)
		}
		cGroupIDToPath[byteOrder.Uint64(handle.Bytes())] = path
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to find cgroup id: %v", err)
	}
	if p, ok := cGroupIDToPath[cgroupId]; ok {
		return p, nil
	}

	cGroupIDToPath[cgroupId] = unknownPath
	return cGroupIDToPath[cgroupId], nil
}

//get CgroupPath from the containerID
func GetCGroupPathFromContainerID(nameOrID string) string {

	data, err := containers.Inspect(*ctx, nameOrID, nil)
	if err != nil {
		log.Fatalf("cannot get container:%v", err)
	}
	CGroupPath := data.State.CgroupPath

	return cgroupPath + CGroupPath

}

//get CGroupID of A file using inode
func getCGroupIDOfAFile(filePath string) (uint64, error) {
	var stat syscall.Stat_t
	if err := syscall.Stat(filePath, &stat); err != nil {
		return 0, fmt.Errorf("Failed to get Inode of file:'%s': %w", filePath, err)
	}
	return stat.Ino, nil
}

func GetCGroupIDFromCGroupPath(CGroupPath string) (uint64, error) {
	ino_val, err := getCGroupIDOfAFile(CGroupPath)
	if err != nil {
		return 0, fmt.Errorf("Failed to get Inode of file:'%s': %w", CGroupPath, err)
	}
	return ino_val, nil
}

// func (collector.ContainerEnergy) GetNameFromcGgroupID() (cgroupID uint64) {

// }
