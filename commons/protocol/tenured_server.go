package protocol

import (
	"errors"
	"github.com/ihaiker/tenured-go-server/commons/future"
	"github.com/ihaiker/tenured-go-server/commons/remoting"
	"github.com/sirupsen/logrus"
	"reflect"
	"sync/atomic"
	"time"
)

const (
	S_RUNING = uint32(0)
	S_CLOSED = uint32(1)
)

type TenuredServer struct {
	server           *remoting.RemotingServer
	responseTables   map[uint32]*future.SetFuture
	commandProcesser map[uint16]*tenuredCommandRunner
	*remoting.HandlerWrapper

	closeStatus uint32
}

func (this *TenuredServer) IsActive() bool {
	return atomic.LoadUint32(&this.closeStatus) == S_RUNING
}

func (this *TenuredServer) Invoke(channel string, command *TenuredCommand, timeout time.Duration) (*TenuredCommand, error) {
	if !this.IsActive() {
		return nil, &TenuredError{Code: remoting.ErrClosed.String(), Message: "closed"}
	}
	requestId := command.Id
	responseFuture := future.Set()
	this.responseTables[requestId] = responseFuture

	if err := this.server.SendTo(channel, command, timeout); err != nil {
		logrus.Debug("send %d error:", requestId, err)
		delete(this.responseTables, requestId)
		return nil, err
	} else {
		response, err := responseFuture.GetWithTimeout(timeout)
		delete(this.responseTables, requestId)
		if err != nil {
			return nil, err
		}
		if responseCommand, match := response.(*TenuredCommand); !match {
			return nil, errors.New("response type error：" + reflect.TypeOf(response).Name())
		} else {
			return responseCommand, nil
		}
	}
}

func (this *TenuredServer) AsyncInvoke(channel string, command *TenuredCommand, timeout time.Duration,
	callback func(tenuredCommand *TenuredCommand, err error)) {
	if !this.IsActive() {
		callback(nil, &TenuredError{Code: remoting.ErrClosed.String(), Message: "closed"})
		return
	}
	requestId := command.Id
	responseFuture := future.Set()
	this.responseTables[requestId] = responseFuture

	this.server.SyncSendTo(channel, command, timeout, func(err error) {
		if err != nil {
			logrus.Debug("async send %d error", requestId)
			callback(nil, err)
			delete(this.responseTables, requestId)
		} else {
			logrus.Debug("async send %d error", requestId)
		}
	})

	go func() {
		response, err := responseFuture.GetWithTimeout(timeout)
		delete(this.responseTables, requestId)

		if err != nil {
			callback(nil, err)
			return
		}

		if responseCommand, match := response.(*TenuredCommand); !match {
			callback(nil, errors.New("response type error："+reflect.TypeOf(response).Name()))
		} else {
			callback(responseCommand, nil)
		}
	}()
}

func (this *TenuredServer) RegisterCommandProcesser(code uint16, processer TenuredCommandProcesser, poolSize int) {
	this.commandProcesser[code] = &tenuredCommandRunner{
		process: processer, poolSize: poolSize,
	}
}

func (this *TenuredServer) makeAck(channel remoting.RemotingChannel, command *TenuredCommand) {
	response := NewACK(command.Id)
	if err := channel.Write(response, time.Second*7); err != nil {
		if remoting.IsRemotingError(err, remoting.ErrClosed) {
			return
		}
		logrus.Warn("send ack error: %s", err.Error())
	}
}

func (this *TenuredServer) onCommandProcesser(channel remoting.RemotingChannel, command *TenuredCommand) {
	this.makeAck(channel, command)
	if processRunner, has := this.commandProcesser[command.Code]; has {
		processRunner.onCommand(channel, command)
	}
}

func (this *TenuredServer) OnMessage(channel remoting.RemotingChannel, msg interface{}) {
	command := msg.(*TenuredCommand)
	if command.IsACK() {
		requestId := command.Id
		if f, has := this.responseTables[requestId]; has {
			f.Set(command)
		}
		return
	} else {
		//TODO 用管理优化
		go this.onCommandProcesser(channel, command)
	}
}

//发送心跳包
func (this *TenuredServer) OnIdle(channel remoting.RemotingChannel) {
	if err := channel.Write(NewIdle(), time.Second*3); err != nil {
		if remoting.IsRemotingError(err, remoting.ErrClosed) {
			return
		}
		logrus.Warnf("send %s idle error: %v", channel.RemoteAddr(), err)
	}
}

func (this *TenuredServer) OnClose(channel remoting.RemotingChannel) {
	this.fastFailChannel(channel)
}

func (this *TenuredServer) fastFailChannel(channel remoting.RemotingChannel) {
	//TODO 去除有的消息等待
}

func (this *TenuredServer) Start() error {
	return this.server.Start()
}

func (this *TenuredServer) waitRequest(interrupt bool) {
	if interrupt {
		for _, v := range this.responseTables {
			v.Exception(errors.New(remoting.ErrClosed.String()))
		}
	} else {
		for {
			if len(this.responseTables) == 0 {
				return
			}
			<-time.After(time.Millisecond * 200)
		}
	}
}

func (this *TenuredServer) Shutdown() {
	this.server.Shutdown()
	if atomic.CompareAndSwapUint32(&this.closeStatus, S_RUNING, S_CLOSED) {
		this.waitRequest(false)
	}
}

func (this *TenuredServer) InterruptShutdown() {
	this.server.Shutdown()
	if atomic.CompareAndSwapUint32(&this.closeStatus, S_RUNING, S_CLOSED) {
		this.waitRequest(true)
	}
}

func NewTenuredServer(address string, config *remoting.RemotingConfig) (*TenuredServer, error) {
	if remotingServer, err := remoting.NewRemotingServer(address, config); err != nil {
		return nil, err
	} else {
		remotingServer.SetCoder(&tenuredCoder{config: config})
		server := &TenuredServer{
			server:           remotingServer,
			responseTables:   map[uint32]*future.SetFuture{},
			commandProcesser: map[uint16]*tenuredCommandRunner{},
		}
		remotingServer.SetHandler(server)
		return server, nil
	}
}