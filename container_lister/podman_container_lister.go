package container_lister

import (
	"encoding/json"
	"fmt"
	podapi "github.com/containers/libpod/libpod"
	"io/ioutil"
//	corev1 "k8s.io/api/core/v1"
	"log"
	"net/http"
	"os"
)

type PodmanContainerLister struct{}

const (
	//	saPath         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
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
	//	podNameTag       = "pod"
	//	namespaceTag     = "namespace"
)

func init() {
	nodeName := os.Getenv(EdgeDeviceEnv)
	if len(nodeName) == 0 {
		nodeName = "localhost"
	}
	port := os.Getenv(PodmanServicePortEnv)
	if len(port) == 0 {
		port = "10250"
	}
	containerUrl = "https://" + nodeName + ":" + port + "/libpod/containers/json"
	//metricsUrl = "https://" + nodeName + ":" + port + "/metrics/resource"  it is not required as now we can get the list of containers directly instead of k8 metrics.
}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from %q: %v", url, err)
	}
	return resp, err
}
func (k *PodmanContainerLister) ListContainers() (*[]podapi.Container, error) {

	resp, err := httpGet(containerUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get response: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}
	//podList := corev1.PodList{}
	containerList := []podapi.Container
	err = json.Unmarshal(body, &containerList)
	if err != nil {
		log.Fatalf("failed to parse response body: %v", err)
	}

	return &containerList, nil
}

func main() {

}
