package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/play-with-docker/play-with-docker/docker"
	"github.com/play-with-docker/play-with-docker/event"
	"github.com/play-with-docker/play-with-docker/pwd/types"
	"github.com/play-with-docker/play-with-docker/router"
	"github.com/play-with-docker/play-with-docker/storage"
)

type SystemPorts struct {
	Instance string `json:"instance"`
	Ports    []int  `json:"ports"`
}

type checkSystemPorts struct {
	event   event.EventApi
	factory docker.FactoryApi
	cli     *http.Client
	storage storage.StorageApi
	cache   *lru.Cache
}

var CheckSystemPortsEvent event.EventType

func init() {
	CheckSystemPortsEvent = event.EventType("instance system ports")
}

func (t *checkSystemPorts) Name() string {
	return "CheckSystemPorts"
}

func (t *checkSystemPorts) Run(ctx context.Context, instance *types.Instance) error {
	host := router.EncodeHost(instance.SessionId, instance.IP, router.HostOpts{EncodedPort: 4401})
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", host), nil)
	if err != nil {
		log.Printf("Could not create request to get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
		return fmt.Errorf("Could not create request to get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
	}
	req.Header.Set("X-Proxy-Host", "l2")
	resp, err := t.cli.Do(req)
	if err != nil {
		log.Printf("Could not get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
		return fmt.Errorf("Could not get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
	}
	if resp.StatusCode != 200 {
		log.Printf("Could not get stats of windows instance with IP %s. Got status code: %d\n", instance.IP, resp.StatusCode)
		return fmt.Errorf("Could not get stats of windows instance with IP %s. Got status code: %d\n", instance.IP, resp.StatusCode)
	}

	ports := make([]int, 0)
	err = json.NewDecoder(resp.Body).Decode(&ports)
	if err != nil {
		log.Printf("Could not get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
		return fmt.Errorf("Could not get stats of windows instance with IP %s. Got: %v\n", instance.IP, err)
	}

	t.event.Emit(CheckPortsEvent, instance.SessionId, DockerPorts{Instance: instance.Name, Ports: ports})

	return nil
}

func NewCheckSystemPorts(e event.EventApi, f docker.FactoryApi, s storage.StorageApi) *checkSystemPorts {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   1 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConnsPerHost: 5,
		Proxy:               proxyHost,
	}
	cli := &http.Client{
		Transport: transport,
	}
	log.Println(">> CheckSystemPorts <<")
	c, _ := lru.New(5000)
	return &checkSystemPorts{event: e, factory: f, cli: cli, storage: s, cache: c}
}
