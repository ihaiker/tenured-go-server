package linker

import (
	"github.com/ihaiker/tenured-go-server/commons/mixins"
	"github.com/ihaiker/tenured-go-server/commons/nets"
	"github.com/ihaiker/tenured-go-server/commons/remoting"
	"github.com/ihaiker/tenured-go-server/engine"
	"github.com/ihaiker/tenured-go-server/services"
)

type linkerConfig struct {
	//注册服务的前缀，所有系统保持一致
	Prefix string `json:"prefix" yaml:"prefix"`

	//数据存储位置
	Data string `json:"data" yaml:"data"`

	Logs *services.Logs `json:"logs" json:"logs"`

	Registry *services.Registry `json:"registry" yaml:"registry"` //注册中心

	Tcp *services.Tcp `json:"tcp" yaml:"tcp"`

	Executors services.Executors `json:"executors" yaml:"executors"`

	Engine *engine.StoreEngineConfig `json:"engine" yaml:"engine"`
}

func NewLinkerConfig() *linkerConfig {
	return &linkerConfig{
		Prefix: mixins.Get(mixins.KeyServerPrefix, mixins.ServerPrefix),
		Data:   mixins.Get(mixins.KeyDataPath, mixins.DataPath),
		Logs: &services.Logs{
			Level:  "info",
			Path:   mixins.Get(mixins.KeyDataPath, mixins.DataPath) + "/logs/linker.log",
			Output: "stdout",
		},
		Registry: &services.Registry{
			Address: mixins.Get(mixins.KeyRegistry, mixins.Registry),
		},
		Tcp: &services.Tcp{
			IpAndPort: &nets.IpAndPort{
				Port: mixins.PortLinker,
			},
			RemotingConfig: remoting.DefaultConfig(),
		},
		Engine: &engine.StoreEngineConfig{
			Type: "leveldb",
		},
		Executors: services.Executors(map[string]string{}),
	}
}
