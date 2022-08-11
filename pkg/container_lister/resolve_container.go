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
	"errors"
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
	unknownPath            string = "unknown"
)

var (
	containerLister            PodmanContainerLister
	cGroupIDToContainerIDCache = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}

	cGroupIDToPath = map[uint64]string{}

	// containerID
	containerIDToCgroup_id = map[string]uint64{}
	re                     = regexp.MustCompile(`libpod-(.*?)\.scope`)
	cgroupPathInitial      = "/sys/fs/cgroup"
	byteOrder              binary.ByteOrder
	ctx                    *context.Context
)

func init() {
	ctx = StartingPodmanSocket()
	byteOrder = bpf.GetHostByteOrder()
	containerLister = PodmanContainerLister{}
	updateListContainerCache("", false)
}

func GetSystemProcessName() string {
	return systemProcessName
}

func GetContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	if info == nil {
		return "unknown", err
	}
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
		return info, fmt.Errorf("failed to find the cGroup id: %v", err)
	}
	if containerID == systemProcessName {
		return nil, fmt.Errorf("failed to find the cGroup id: %v", errors.New("system process"))
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		return i, nil
	}

	updateListContainerCache(containerID, true)
	if info, ok := containerIDToContainerInfo[containerID]; ok {
		return info, nil
	}

	containerIDToContainerInfo[containerID] = info
	return containerIDToContainerInfo[containerID], nil
}

// updateListPodCache updates cache info with all pods and optionally
// stops the loop when a given container ID is found
func updateListContainerCache(targetContainerID string, stopWhenFound bool) {

	containers, err := containerLister.ListContainers(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, container := range containers {
		info := &ContainerInfo{
			ContainerName: container.Names[0],
			ContainerID:   container.ID,
		}
		containerID := info.ContainerID

		Cgroup_path := GetCGroupPathFromContainerID(containerID) // using default method
		Cgroup_id, err := GetCGroupIDFromCGroupPath(cgroupPathInitial + Cgroup_path)

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

	var err error
	var path string
	if path, err = GetPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	if re.MatchString(path) {
		sub := re.FindAllString(path, -1)
		for _, element := range sub {
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
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18
// (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func GetPathFromcGroupID(cgroupId uint64) (string, error) {

	//check if cgroup if has a container associated , if ok then get path
	if p, ok := cGroupIDToContainerIDCache[cgroupId]; ok {
		path := GetCGroupPathFromContainerID(p)
		cGroupIDToPath[cgroupId] = path
		return cGroupIDToPath[cgroupId], nil
	}

	if p, ok := cGroupIDToPath[cgroupId]; ok {
		return p, nil
	}

	err := filepath.WalkDir(cgroupPathInitial, func(path string, dentry fs.DirEntry, err error) error {
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

func GetCGroupPathFromContainerID(nameOrID string) string {

	data, err := containers.Inspect(*ctx, nameOrID, nil)
	if err != nil {
		log.Printf("cannot get container:%v", err)
		return ""
	}
	CGroupPath := data.State.CgroupPath
	return CGroupPath + "/container"

}

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
		return 0, fmt.Errorf("failed to get Inode of file:'%s': %w", CGroupPath, err)
	}
	return ino_val, nil
}
