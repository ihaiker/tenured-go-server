package tenant

import (
	"fmt"
	"github.com/ihaiker/tenured-go-server/commons"
	"github.com/ihaiker/tenured-go-server/engine"
	"github.com/ihaiker/tenured-go-server/registry"
	"github.com/ihaiker/tenured-go-server/registry/cache"
	"github.com/ihaiker/tenured-go-server/registry/load_balance"
	"github.com/ihaiker/tenured-go-server/registry/plugins"
	"github.com/ihaiker/tenured-go-server/services/tenant/controller"
	"hash/crc64"
)

type TenantServer struct {
	config     *TenantConfig
	httpServer *ctl.HttpServer

	reg            registry.ServiceRegistry
	serviceManager *commons.ServiceManager
	registryPlugin registry.Plugins

	storeClientLoadBalance load_balance.LoadBalance
}

func (this *TenantServer) initRegistry() error {
	registryPlugin, err := plugins.GetRegistryPlugins(this.config.Registry.Address)
	if err != nil {
		return err
	}
	if reg, err := registryPlugin.Registry(); err != nil {
		return err
	} else {
		this.reg = cache.NewCacheRegistry(reg)
		this.serviceManager.Add(reg)
	}
	this.registryPlugin = registryPlugin
	return nil
}

func (this *TenantServer) startRegistry() error {
	if serverInstance, err := this.registryPlugin.Instance(this.config.Registry.Attributes); err != nil {
		return err
	} else {
		address, _ := this.config.HTTP.GetAddress()
		serverInstance.Name = this.config.Prefix + "_tenant"
		serverInstance.Id = fmt.Sprintf("%v", crc64.Checksum([]byte(address), crc64.MakeTable(crc64.ECMA)))
		serverInstance.Address = address
		if err := this.reg.Register(serverInstance); err != nil {
			return err
		}
		return nil
	}
}

func (this *TenantServer) initClientPlugin() error {
	storeName := this.config.Prefix + "_store"
	if clientPlugin, err := engine.GetStoreClientPlugin(storeName, this.config.StoreClient, this.reg); err != nil {
		return err
	} else {
		this.storeClientLoadBalance = clientPlugin.LoadBalance()
		this.serviceManager.Add(this.storeClientLoadBalance)
	}
	return nil
}

func (this *TenantServer) initHttpServer() error {
	httpAddress, err := this.config.HTTP.GetAddress()
	if err != nil {
		return err
	}
	this.httpServer = ctl.NewHttpServer(httpAddress, this.storeClientLoadBalance)
	this.serviceManager.Add(this.httpServer)
	return nil
}

func (this *TenantServer) init() error {
	if err := this.initRegistry(); err != nil {
		return err
	}
	if err := this.initClientPlugin(); err != nil {
		return err
	}
	if err := this.initHttpServer(); err != nil {
		return err
	}
	return nil
}

func (this *TenantServer) Start() error {
	logger.Info("start console http server")
	if err := this.init(); err != nil {
		return err
	}
	if err := this.serviceManager.Start(); err != nil {
		return err
	}
	return this.startRegistry()
}

func (this *TenantServer) Shutdown(interrupt bool) {
	logger.Info("stop console http server")
	this.serviceManager.Shutdown(interrupt)
}

func newTenantServer(config *TenantConfig) (*TenantServer, error) {
	server := &TenantServer{
		config:         config,
		serviceManager: commons.NewServiceManager(),
	}
	return server, nil
}