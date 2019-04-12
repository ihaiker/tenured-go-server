package cache

import (
	"github.com/ihaiker/tenured-go-server/commons/registry"
	"reflect"
)

type CacheServiceRegistry struct {
	reg registry.ServiceRegistry

	cache map[uintptr]registry.RegistryNotifyListener

	serverCache map[string][]*registry.ServerInstance
}

func (this *CacheServiceRegistry) Register(serverInstance *registry.ServerInstance) error {
	return this.reg.Register(serverInstance)
}

func (this *CacheServiceRegistry) Unregister(serverId string) error {
	return this.reg.Unregister(serverId)
}

func (this *CacheServiceRegistry) Subscribe(serverName string, listener registry.RegistryNotifyListener) error {
	pointer := reflect.ValueOf(listener).Pointer()
	if _, has := this.cache[pointer]; has {
		return nil
	}
	newLis := registry.RegistryNotifyListener(func(status registry.RegistionStatus, serverInstances []*registry.ServerInstance) {
		listener(status, serverInstances)
	L1:
		for _, serverInstance := range serverInstances {
			if cacheServerInstances, has := this.serverCache[serverInstance.Name]; has {
				for _, cacheServerInstance := range cacheServerInstances {
					if cacheServerInstance.Id == serverInstance.Id {
						cacheServerInstance.Status = serverInstance.Status
						continue L1
					}
				}
			}
		}
	})
	this.cache[pointer] = newLis
	return this.reg.Subscribe(serverName, newLis)
}

func (this *CacheServiceRegistry) Unsubscribe(serverName string, listener registry.RegistryNotifyListener) error {
	pointer := reflect.ValueOf(listener).Pointer()
	if l, has := this.cache[pointer]; has {
		return this.reg.Unsubscribe(serverName, l)
	} else {
		return nil
	}
}

func (this *CacheServiceRegistry) Lookup(serverName string, tags []string) ([]*registry.ServerInstance, error) {
	if ss, has := this.serverCache[serverName]; has {
		return ss, nil
	} else {
		ss, err := this.reg.Lookup(serverName, tags)
		if err == nil {
			this.serverCache[serverName] = ss
		}
		return ss, err
	}
}

func NewCacheRegistry(reg registry.ServiceRegistry) registry.ServiceRegistry {
	return &CacheServiceRegistry{
		reg:         reg,
		cache:       map[uintptr]registry.RegistryNotifyListener{},
		serverCache: map[string][]*registry.ServerInstance{},
	}
}
