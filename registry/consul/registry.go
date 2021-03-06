package consul

import (
	"fmt"
	"github.com/hashicorp/consul/api"
	"github.com/ihaiker/tenured-go-server/registry"
	"sync"

	"net"
	"strconv"
)

//服务注册监听者
type subscriber struct {
	listeners map[string]registry.RegistryNotifyListener
	services  map[string]*registry.ServerInstance
	closeChan chan struct{}
}

func (this *subscriber) notify(serverInstance []*registry.ServerInstance) {
	for _, lis := range this.listeners {
		for _, si := range serverInstance {
			logger.Infof("notify registry name=%s, id=%s, status=%s", si.Name, si.Id, si.Status)
		}
		lis(serverInstance)
	}
}

func (this *subscriber) close() {
	close(this.closeChan)
}

type ConsulServiceRegistry struct {
	lock   *sync.Mutex
	client *api.Client
	config *ConsulConfig
	//订阅信息
	subscribes map[string]*subscriber
}

func (this *ConsulServiceRegistry) Start() error {
	return nil
}

func (this *ConsulServiceRegistry) Shutdown(interrupt bool) {
	for name, ch := range this.subscribes {
		ch.close()
		delete(this.subscribes, name)
	}
}

func (this *ConsulServiceRegistry) Register(serverInstance *registry.ServerInstance) error {
	logger.Infof("register %s(%s) : %s", serverInstance.Name, serverInstance.Address, serverInstance.Id)
	attrs := serverInstance.PluginAttrs.(*ConsulServerAttrs)
	if host, portStr, err := net.SplitHostPort(serverInstance.Address); err != nil {
		return err
	} else if port, err := strconv.Atoi(portStr); err != nil {
		return err
	} else {
		check := &api.AgentServiceCheck{ // 健康检查
			Interval:                       attrs.Interval,
			Timeout:                        attrs.RequestTimeout,
			DeregisterCriticalServiceAfter: attrs.Deregister,
		}
		switch attrs.CheckType {
		case "http":
			check.HTTP = "http://" + serverInstance.Address + attrs.Health
		case "tcp":
			check.TCP = serverInstance.Address
		}

		reg := &api.AgentServiceRegistration{
			ID:      serverInstance.Id,   // 服务节点的名称
			Name:    serverInstance.Name, // 服务名称
			Meta:    serverInstance.Metadata,
			Tags:    serverInstance.Tags,
			Address: host, Port: port, // 服务 IP:端口
			Check: check,
		}
		return this.client.Agent().ServiceRegister(reg)
	}
}

func (this *ConsulServiceRegistry) Unregister(serverId string) error {
	logger.Info("unregister ", serverId)
	return this.client.Agent().ServiceDeregister(serverId)
}

func (this *ConsulServiceRegistry) convertService(serverName string, service *api.ServiceEntry) *registry.ServerInstance {
	status := service.Checks.AggregatedStatus()
	if status == api.HealthPassing {
		status = registry.StatusOK
	} else {
		status = registry.StatusCritical
	}
	return &registry.ServerInstance{
		Id:       service.Service.ID,
		Name:     serverName,
		Metadata: service.Service.Meta,
		Address:  fmt.Sprintf("%s:%d", service.Service.Address, service.Service.Port),
		Tags:     service.Service.Tags,
		Status:   status,
	}
}

func (this *ConsulServiceRegistry) loadSubscribeHealth(serverName string) {
	defer func() {
		if e := recover(); e != nil {
			logger.Warnf("close subscribe(%s) error: %v", serverName, e)
		}
	}()
	logger.Debug("start subscribe server health: ", serverName)

	waitIndex := uint64(0)
	healthWaitTime := this.config.HealthWaitTime()

	for {
		subscribe, has := this.subscribes[serverName]
		if !has {
			return
		}
		select {
		case <-subscribe.closeChan:
			return
		default:
			serviceEntries, queryMeta, err := this.client.Health().Service(serverName, "", false,
				&api.QueryOptions{
					WaitIndex: waitIndex,      //同步点，这个调用将一直阻塞，直到有新的更新,
					WaitTime:  healthWaitTime, //此次请求等待时间，此处设置防止携程阻死
					//UseCache:  true, MaxAge:time.Second*5
					AllowStale: true,
				})
			if err != nil {
				logger.Warn("load registry error:", err)
				continue
			}
			if waitIndex == queryMeta.LastIndex {
				continue
			}

			subscribe, has = this.subscribes[serverName]
			if !has {
				return
			}
			logger.Debug("registry service changed : ", serverName)

			notifies := make([]*registry.ServerInstance, 0)

			currentServices := map[string]*registry.ServerInstance{}
			for _, serviceEntry := range serviceEntries {
				serverInstance := this.convertService(serverName, serviceEntry)
				currentServices[serverInstance.Id] = serverInstance

				if old, has := subscribe.services[serverInstance.Id]; has {
					if old.Status != serverInstance.Status {
						notifies = append(notifies, serverInstance)
					}
				} else {
					notifies = append(notifies, serverInstance)
				}
			}

			for _, serverInstance := range subscribe.services {
				if _, has := currentServices[serverInstance.Id]; !has {
					serverInstance.Status = registry.StatusDown
					notifies = append(notifies, serverInstance)
				}
			}
			if len(notifies) != 0 {
				subscribe.notify(notifies)
			}
			subscribe.services = currentServices
			waitIndex = queryMeta.LastIndex
		}
	}
}

func (this *ConsulServiceRegistry) Subscribe(serverName string, listener registry.RegistryNotifyListener) error {
	this.lock.Lock()
	defer this.lock.Unlock()

	logger.Info("Subscribe ", serverName)
	if this.addSubscribe(serverName, listener) {
		go this.loadSubscribeHealth(serverName)
	}
	return nil
}

func (this *ConsulServiceRegistry) Unsubscribe(serverName string, listener registry.RegistryNotifyListener) error {
	this.lock.Lock()
	defer this.lock.Unlock()

	logger.Info("Unsubscribe ", serverName, ",fn ", listener)
	if this.removeSubscribe(serverName, listener) {
		if sub, has := this.subscribes[serverName]; has {
			sub.close()
			delete(this.subscribes, serverName)
		}
	}
	return nil
}

func (this *ConsulServiceRegistry) Lookup(serverName string, tags []string) ([]*registry.ServerInstance, error) {
	if services, _, err := this.client.Health().
		ServiceMultipleTags(serverName, tags, false, &api.QueryOptions{}); err != nil {
		return nil, err
	} else {
		serverInstances := make([]*registry.ServerInstance, len(services))
		for i := 0; i < len(services); i++ {
			serverInstances[i] = this.convertService(serverName, services[i])
		}
		return serverInstances, nil
	}
}

func (this *ConsulServiceRegistry) getOrCreateSubscribe(name string) *subscriber {
	if subInfo, has := this.subscribes[name]; !has {
		subInfo = &subscriber{
			listeners: map[string]registry.RegistryNotifyListener{},
			services:  map[string]*registry.ServerInstance{},
			closeChan: make(chan struct{}),
		}
		this.subscribes[name] = subInfo
	}
	return this.subscribes[name]
}

//@return 返回是否是此服务的第一个监听器
func (this *ConsulServiceRegistry) addSubscribe(name string, listener registry.RegistryNotifyListener) bool {
	sets := this.getOrCreateSubscribe(name)
	from := len(sets.listeners)
	pointer := registry.NotifyPointer(listener)
	sets.listeners[pointer] = listener
	return from == 0 && len(sets.listeners) == 1
}

//@return 是否是次服务的最后一个监听器
func (this *ConsulServiceRegistry) removeSubscribe(name string, listener registry.RegistryNotifyListener) bool {
	if sets, has := this.subscribes[name]; has {
		pointer := registry.NotifyPointer(listener)
		from := len(sets.listeners)
		delete(sets.listeners, pointer)
		return from == 1 && len(sets.listeners) == 0
	} else {
		return false
	}
}

func newRegistry(pluginConfig *registry.PluginConfig) (*ConsulServiceRegistry, error) {
	config := &ConsulConfig{config: pluginConfig}
	serviceRegistry := &ConsulServiceRegistry{
		lock:       new(sync.Mutex),
		config:     config,
		subscribes: map[string]*subscriber{},
	}

	consulApiCfg := api.DefaultConfig()
	consulApiCfg.Scheme = config.Scheme()
	consulApiCfg.Address = config.Address()
	consulApiCfg.Datacenter = config.Datacenter()
	consulApiCfg.Token = config.Token()

	if client, err := api.NewClient(consulApiCfg); err != nil {
		return nil, err
	} else {
		serviceRegistry.client = client
	}
	return serviceRegistry, nil
}
