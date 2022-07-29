package container_lister

import (
	"context"
	"fmt"
	"log"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

const (
	EdgeDeviceEnv        = "NODE_NAME"
	PodmanServicePortEnv = "KUBELET_PORT"
)

var (
	containerUrl, metricsUrl string

	EdgeDeviceCpuUsageMetricName = "node_cpu_usage_seconds_total"
	EdgeDeviceMemUsageMetricName = "node_memory_working_set_bytes"
	containerCpuUsageMetricName  = "container_cpu_usage_seconds_total"
	containerMemUsageMetricName  = "container_memory_working_set_bytes"
	containerStartTimeMetricName = "container_start_time_seconds"

	containerNameTag = "container"
)

type PodmanContainerLister struct{}

//type ContainerData struct {
//}

func StartingPodmanSocket() *context.Context {
	fmt.Println("Starting")
	ctx, err := bindings.NewConnection(context.Background(), "unix:/run/podman/podman.sock")
	if err != nil {
		log.Fatalf("cannot connect to podman :%v", err)
	}
	return &ctx
}

//6
func (k *PodmanContainerLister) ListContainers(contxt *context.Context) ([]entities.ListContainer, error) {
	fmt.Println(6)
	//ctx := StartingPodmanSocket()
	containerList, err := containers.List(*contxt, nil)

	if err != nil {
		log.Fatalf("cannot get pods:%v", err)
	}
	return containerList, nil
}

func main() {

}
