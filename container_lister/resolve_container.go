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
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	bpf "github.com/iovisor/gobpf/bcc"
)

type ContainerInfo struct {
	//	PodName       string
	ContainerName string
	//	Namespace     string
}

const (
	systemProcessName      string = "system_processes"
	systemProcessNamespace string = "system"
	containerIDPredix      string = "cri-o://"
	unknownPath            string = "unknown"
)

var (
	contaianerLister           PodmanContainerLister
	cGroupIDToContainerIDCache = map[uint64]string{}
	containerIDToContainerInfo = map[string]*ContainerInfo{}
	cGroupIDToPath             = map[uint64]string{}
	re                         = regexp.MustCompile(`crio-(.*?)\.scope`)
	cgroupPath                 = "/sys/fs/cgroup"
	byteOrder                  binary.ByteOrder
)

func init() {
	byteOrder = bpf.GetHostByteOrder()
	contaianerLister = PodmanContainerLister{}
	updateListContainerCache("", false)
}

func GetSystemProcessName() string {
	return systemProcessName
}

//func GetPodNameFromcGgroupID(cGroupID uint64) (string, error) {
//	info, err := getContainerInfoFromcGgroupID(cGroupID)
//	return info.PodName, err
//}

//func GetPodNameSpaceFromcGgroupID(cGroupID uint64) (string, error) {
//	info, err := getContainerInfoFromcGgroupID(cGroupID)
//	return info.Namespace, err
//}

func GetContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.ContainerName, err
}

//func GetPodMetrics() (containerCPU map[string]float64, containerMem map[string]float64, nodeCPU float64, nodeMem float64, retErr error) {
//	return podLister.ListMetrics()
//}

func getContainerInfoFromcGgroupID(cGroupID uint64) (*ContainerInfo, error) {
	var err error
	var containerID string
	info := &ContainerInfo{
		//PodName:   systemProcessName,
		//Namespace: systemProcessNamespace,
	}

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
	containers, err := contaianerLister.ListContainers()
	if err != nil {
		log.Fatal(err)
	}
	for _, container := range containers {
		//statuses := container.Status
		//for _, status := range statuses {
		info := &ContainerInfo{
			//PodName:       pod.Name,
			//Namespace:     pod.Namespace,
			ContainerName: container.Names[0],
		}
		containerID := strings.Trim(container.ID, containerIDPredix)
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
	if path, err = getPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	if re.MatchString(path) {
		sub := re.FindAllString(path, -1)
		for _, element := range sub {
			if strings.Contains(element, "-conmon-") || strings.Contains(element, ".service") {
				return "", fmt.Errorf("process cGroupID %d is not in a kubernetes pod", cGroupID)
			} else if strings.Contains(element, "crio") {
				containerID := strings.Trim(element, "crio-")
				containerID = strings.Trim(containerID, ".scope")
				cGroupIDToContainerIDCache[cGroupID] = containerID
				return cGroupIDToContainerIDCache[cGroupID], nil
			}
		}
	}
	cGroupIDToContainerIDCache[cGroupID] = systemProcessName
	return cGroupIDToContainerIDCache[cGroupID], fmt.Errorf("failed to find container with cGroup id: %v", cGroupID)
}

// getPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18+ (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
func getPathFromcGroupID(cgroupId uint64) (string, error) {
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

func GetCGroupPathFromContainerID(ctx *context.Context, nameOrID string) string {
	data, err := containers.Inspect(*ctx, nameOrID, nil)
	if err != nil {
		log.Fatalf("cannot get container:%v", err)
	}
	CGroupPath := data.State.CgroupPath
	return CGroupPath
}

func getInodeOfAFile(fileName string) uint64 {
	var stat syscall.Stat_t
	if err := syscall.Stat(fileName, &stat); err != nil {
		panic(err)
	}
	return stat.Ino
}

func GetInodefOfCGroup(CGroupPath string) uint64 {
	ino_val := getInodeOfAFile(CGroupPath)
	return ino_val
}
