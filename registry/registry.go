package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	timeout  time.Duration
	mu       sync.Mutex
	services map[string]*ServiceInfo
}

type ServiceInfo struct {
	Addr  string
	start time.Time
}

const (
	defaultPath    = "/_cleanwk_/registry"
	defaultTimeout = time.Minute * 5
)

func New(timeout time.Duration) *Registry {
	return &Registry{
		services: make(map[string]*ServiceInfo),
		timeout:  timeout,
	}
}

var DefaultRegister = New(defaultTimeout)

//添加服务
func (r *Registry) addService(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.services[addr]
	if s == nil {
		r.services[addr] = &ServiceInfo{
			Addr:  addr,
			start: time.Now(),
		}
	} else {
		s.start = time.Now()
	}
}

// 返回可用状态的服务列表
func (r *Registry) getAliveServices() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, service := range r.services {
		if r.timeout == 0 || service.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.services, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

func (r *Registry) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case "GET":
		writer.Header().Set("X-Cleanwk-Servers", strings.Join(r.getAliveServices(), ","))
	case "POST":
		addr := request.Header.Get("X-Cleanwk-Servers")
		if addr == "" {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.addService(addr)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultRegister.HandleHTTP(defaultPath)
}

func HeartBeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-cleanwk-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil
}
