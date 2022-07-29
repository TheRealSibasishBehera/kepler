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
	fmt.Println("container_lister init function over")
}

func GetSystemProcessName() string {
	return systemProcessName
}

//2
func GetContainerNameFromcGgroupID(cGroupID uint64) (string, error) {
	fmt.Println("2 GetContainerNameFromcGgroupID called")
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	if info == nil {
		return "unknown", err
	}
	return info.ContainerName, err
}

func GetContainerIDFromcGgroupID(cGroupID uint64) (string, error) {
	fmt.Println(" GetContainerNameFromcGgroupID called")
	info, err := getContainerInfoFromcGgroupID(cGroupID)
	return info.ContainerID, err
}

//3
func getContainerInfoFromcGgroupID(cGroupID uint64) (*ContainerInfo, error) {
	fmt.Println("3 getContainerInfoFromcGgroupID called")
	var err error
	var containerID string
	info := &ContainerInfo{}

	if containerID, err = getContainerIDFromcGroupID(cGroupID); err != nil {
		fmt.Println("3 getContainerInfoFromcGgroupID called over")
		fmt.Println("3 info 1: %v", info)
		return info, fmt.Errorf("failed to find the cGroup id: %v", err)
	}
	fmt.Println("3 retured from getContainerIDFromcGroupID withot error ID : ", containerID)
	if containerID == systemProcessName {
		//this means its not a container
		fmt.Println("3 getContainerInfoFromcGgroupID called over and system proc 2")
		return nil, fmt.Errorf("failed to find the cGroup id: %v", errors.New("system process"))
	}

	if i, ok := containerIDToContainerInfo[containerID]; ok {
		fmt.Println("3 ok")
		return i, nil
	}

	// update cache info and stop loop if container id found
	//tries to get info
	fmt.Println("3 call Update")
	updateListContainerCache(containerID, true)
	fmt.Println("called update now ")
	if info, ok := containerIDToContainerInfo[containerID]; ok {
		fmt.Println("3 getContainerInfoFromcGgroupID called over ")
		fmt.Println("3 info 2: %v", info)
		return info, nil
	}

	containerIDToContainerInfo[containerID] = info
	fmt.Println("3 info 3: %v", info)
	return containerIDToContainerInfo[containerID], nil
}

//5
// updateListPodCache updates cache info with all pods and optionally
// stops the loop when a given container ID is found
func updateListContainerCache(targetContainerID string, stopWhenFound bool) {
	fmt.Println("5 updateListContainerCache called")
	//get all the containers
	containers, err := containerLister.ListContainers(ctx)
	fmt.Println("5 ListContainer done ")
	if err != nil {
		log.Fatal(err)
	}

	//range over the containers
	var i = 0
	for _, container := range containers {
		fmt.Println("5 loop no", i)
		info := &ContainerInfo{
			ContainerName: container.Names[0],
			ContainerID:   container.ID,
		}
		i++
		fmt.Println("5 from info Container Name :", info.ContainerName)
		fmt.Println("5 from info Container ID :", info.ContainerID)

		// containerID := strings.Trim(container.ID, containerIDPredix)
		containerID := info.ContainerID

		Cgroup_path := GetCGroupPathFromContainerID(containerID) // usind default method
		fmt.Println("GetCGroupPathFromContainerID done ", Cgroup_path)

		Cgroup_id, err := GetCGroupIDFromCGroupPath(cgroupPathInitial + Cgroup_path)
		fmt.Println("GetCGroupIDFromCGroupPath done", Cgroup_id)

		if err != nil {
			return
		}
		containerIDToCgroup_id[containerID] = Cgroup_id
		cGroupIDToContainerIDCache[Cgroup_id] = containerID

		// fmt.Println("maps for  containerIDToCgroup_id and cGroupIDToContainerIDCache set")

		containerIDToContainerInfo[containerID] = info
		if stopWhenFound && container.ID == targetContainerID {
			fmt.Println("5 updateListContainerCache called over")
			return
		}
	}
	fmt.Println("5 updateListContainerCache called over")
}

//4
func getContainerIDFromcGroupID(cGroupID uint64) (string, error) {
	fmt.Println("4 cgroup ID :", cGroupID)
	fmt.Println("4 getContainerIDFromcGroupID started")

	if id, ok := cGroupIDToContainerIDCache[cGroupID]; ok {
		fmt.Println("4 getContainerIDFromcGroupID over 29")
		return id, nil
	}

	////added another search using
	//p, err1 := GetContainerIDFromcGgroupID(cGroupID)
	//if err1 != nil {
	//	log.Printf("%v\n", err1)
	//} else {
	//	return p, nil
	//}

	var err error
	var path string
	if path, err = GetPathFromcGroupID(cGroupID); err != nil {
		return systemProcessName, err
	}

	if re.MatchString(path) {
		fmt.Println("4 entered match path")
		sub := re.FindAllString(path, -1)
		for _, element := range sub {
			fmt.Println("4 match path element :", element)
			// if strings.Contains(element, "-conmon-") || strings.Contains(element, ".service") {
			// 	return "", fmt.Errorf("process cGroupID %d is not in a kubernetes pod", cGroupID)
			// } else
			if strings.Contains(element, "libpod") {
				containerID := strings.Trim(element, "libpod-")
				fmt.Println("4 containerID 1 trim : ", containerID)
				containerID = strings.Trim(containerID, ".scope")
				fmt.Println("4 containerID 2 trim : ", containerID)
				cGroupIDToContainerIDCache[cGroupID] = containerID
				fmt.Println("4 getContainerIDFromcGroupID over")
				fmt.Println("_________________________________")
				fmt.Println("4 containerID final over: ", containerID)
				fmt.Println("_________________________________")
				return cGroupIDToContainerIDCache[cGroupID], nil
			}
		}
	}
	cGroupIDToContainerIDCache[cGroupID] = systemProcessName
	fmt.Println("4 getContainerIDFromcGroupID over with fail")
	return cGroupIDToContainerIDCache[cGroupID], fmt.Errorf("failed to find container with cGroup id: %v", cGroupID)
}

// GetPathFromcGroupID uses cgroupfs to get cgroup path from id
// it needs cgroup v2 (per https://github.com/iovisor/bpftrace/issues/950) and kernel 4.18
// (https://github.com/torvalds/linux/commit/bf6fa2c893c5237b48569a13fa3c673041430b6c)
//100
func GetPathFromcGroupID(cgroupId uint64) (string, error) {

	fmt.Println("100 GetPathFromcGroupID started")

	//check if cgroup if has a container associated , if ok then get path
	if p, ok := cGroupIDToContainerIDCache[cgroupId]; ok {
		path := GetCGroupPathFromContainerID(p)
		cGroupIDToPath[cgroupId] = path

		fmt.Println("100 GetPathFromcGroupID ended 1")
		fmt.Println(path)
		return cGroupIDToPath[cgroupId], nil
	}

	// //if not found try to update cache
	//container_info, err1 := getContainerInfoFromcGgroupID(cgroupId)
	//if err1 != nil {
	//	fmt.Errorf("%s", err1)
	//}
	//container_id := container_info.ContainerID
	//updateListContainerCache(container_id, true)

	// // check again
	//if p, ok := cGroupIDToContainerIDCache[cgroupId]; ok {
	//	return GetCGroupPathFromContainerID(p), nil
	//}

	if p, ok := cGroupIDToPath[cgroupId]; ok {
		fmt.Println("100 GetPathFromcGroupID ended 2")
		fmt.Println(p)
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
		// fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++")
		// fmt.Println("the cGroupID from byteorder : ", byteOrder.Uint64(handle.Bytes()))
		fmt.Println("100 path from filepath.WalkDir in GetPathFromCGroupID : ", path)
		// fmt.Println("++++++++++++++++++++++++++++++++++++++++++++++")
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("100 failed to find cgroup id: %v", err)
	}
	//checks if passed cgroupid is present
	if p, ok := cGroupIDToPath[cgroupId]; ok {
		fmt.Println("100 GetPathFromcGroupID ended 3")
		return p, nil
	}

	cGroupIDToPath[cgroupId] = unknownPath
	fmt.Println("100 path from failing  : ", cGroupIDToPath[cgroupId])
	fmt.Println("100 GetPathFromcGroupID ended 4")
	return cGroupIDToPath[cgroupId], nil
}

func GetCGroupPathFromContainerID(nameOrID string) string {

	data, err := containers.Inspect(*ctx, nameOrID, nil)
	if err != nil {
		log.Printf("cannot get container:%v", err)
		return ""
	}
	CGroupPath := data.State.CgroupPath
	//relative to /sys/fs/cgroup
	// fmt.Println(CGroupPath)
	return CGroupPath + "/container"

}

//65
func getCGroupIDOfAFile(filePath string) (uint64, error) {
	fmt.Println("65 getCGroupIDOfAFile")
	var stat syscall.Stat_t
	if err := syscall.Stat(filePath, &stat); err != nil {
		return 0, fmt.Errorf("65 Failed to get Inode of file:'%s': %w", filePath, err)
	}
	return stat.Ino, nil
}

//165
func GetCGroupIDFromCGroupPath(CGroupPath string) (uint64, error) {
	fmt.Println("165 getCGroupIDOfAFile")
	ino_val, err := getCGroupIDOfAFile(CGroupPath)
	if err != nil {
		return 0, fmt.Errorf("165 Failed to get Inode of file:'%s': %w", CGroupPath, err)
	}
	return ino_val, nil
}
