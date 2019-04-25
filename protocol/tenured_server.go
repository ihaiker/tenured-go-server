package protocol

import (
	"github.com/ihaiker/tenured-go-server/commons/c8tmap"
	"github.com/ihaiker/tenured-go-server/commons/remoting"
)

type TenuredServer struct {
	tenuredService
	AuthChecker TenuredAuthChecker
	*AuthHeader
}

func (this *TenuredServer) onCommandProcesser(channel remoting.RemotingChannel, command *TenuredCommand) {
	if command.code == REQUEST_CODE_ATUH {
		if err := this.AuthChecker.Auth(channel, command); err != nil {
			logger.Infof("auth channel(%s) error: %s", channel.RemoteAddr(), err.Error())
			this.makeAck(channel, command, nil, err)
		} else {
			logger.Debugf("channel(%s) auth success", channel.RemoteAddr())
			this.makeAck(channel, command, this.AuthHeader, nil)
		}
		return
	} else if this.AuthChecker != nil && !this.AuthChecker.IsAuthed(channel) {
		this.makeAck(channel, command, nil, ErrorNoAuth())
		this.fastFailChannel(channel)
		return
	}
	this.tenuredService.onCommandProcesser(channel, command)
}

func (this *TenuredServer) OnMessage(channel remoting.RemotingChannel, msg interface{}) {
	command := msg.(*TenuredCommand)
	if command.IsACK() {
		requestId := command.id
		if f, has := this.responseTables.Pop(requestId); has {
			f.(*responseTableBlock).future.Set(command)
		}
		return
	} else {
		this.onCommandProcesser(channel, command)
	}
}

func NewTenuredServer(address string, config *remoting.RemotingConfig) (*TenuredServer, error) {
	if config == nil {
		config = remoting.DefaultConfig()
	}
	if remotingServer, err := remoting.NewRemotingServer(address, config); err != nil {
		return nil, err
	} else {
		remotingServer.SetCoder(&tenuredCoder{config: config})
		server := &TenuredServer{
			tenuredService: tenuredService{
				remoting:         remotingServer,
				responseTables:   c8tmap.New(), //map[uint32]*responseTableBlock{},
				commandProcesser: map[uint16]*tenuredCommandRunner{},
			},
			AuthChecker: &ModuleAuthChecker{},
		}
		remotingServer.SetHandler(server)
		return server, nil
	}
}
