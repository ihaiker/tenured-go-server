package store

import (
	"github.com/ihaiker/tenured-go-server/api"
	"github.com/ihaiker/tenured-go-server/commons/mixins"
	"github.com/ihaiker/tenured-go-server/commons/nets"
	"github.com/ihaiker/tenured-go-server/commons/remoting"
	"github.com/ihaiker/tenured-go-server/engine"
	"github.com/ihaiker/tenured-go-server/services"
)

type storeConfig struct {
	Prefix string `json:"prefix" yaml:"prefix"` //注册服务的前缀，所有系统保持一致

	Data string `json:"data" yaml:"data"` //数据存储位置

	Stores []string `json:"stores" yaml:"stores"`

	Logs *services.Logs `json:"logs" json:"logs"`

	Registry *services.Registry `json:"registry" yaml:"registry"` //注册中心

	Tcp *services.Tcp `json:"tcp" yaml:"tcp"`

	Executors map[string]string `json:"executors" yaml:"executors"`

	Engine *engine.StoreEngineConfig `json:"engine" yaml:"engine"`
}

func (this *storeConfig) HasStore(name string) bool {
	for _, store := range this.Stores {
		if store == name {
			return true
		}
	}
	return false
}

func NewStoreConfig() *storeConfig {
	return &storeConfig{
		Prefix: mixins.Get(mixins.KeyServerPrefix, mixins.ServerPrefix),
		Data:   mixins.Get(mixins.KeyDataPath, mixins.DataPath),
		Stores: api.StoreAll,
		Logs: &services.Logs{
			Level:  "info",
			Path:   mixins.Get(mixins.KeyDataPath, mixins.DataPath) + "/logs/store.log",
			Output: "stdout",
		},
		Registry: &services.Registry{
			Address: mixins.Get(mixins.KeyRegistry, mixins.Registry),
		},
		Tcp: &services.Tcp{
			IpAndPort: &nets.IpAndPort{
				Port: mixins.PortStore,
			},
			RemotingConfig: remoting.DefaultConfig(),
		},
		Engine: &engine.StoreEngineConfig{
			Type: "leveldb",
			Attributes: map[string]string{
				"dataPath": mixins.Get(mixins.KeyDataPath, mixins.DataPath),
			},
		},
		Executors: map[string]string{},
	}
}
