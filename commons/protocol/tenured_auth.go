package protocol

import (
	"github.com/ihaiker/tenured-go-server/commons/remoting"
)

const auth_attributes_name = "auth_token"

type TenuredAuthChecker interface {
	Auth(channel remoting.RemotingChannel, command *TenuredCommand) error
	IsAuthed(channel remoting.RemotingChannel) bool
}

type defAuthChecker struct {
}

func (this *defAuthChecker) Auth(channel remoting.RemotingChannel, command *TenuredCommand) error {
	header := &AuthHeader{}
	if err := command.GetHeader(header); err != nil {
		return err
	} else {
		channel.ChannelAttributes()[auth_attributes_name] = "true"
	}

	channel.ChannelAttributes()

	return nil
}

func (this *defAuthChecker) IsAuthed(channel remoting.RemotingChannel) bool {
	attrs := channel.ChannelAttributes()
	if attrs == nil {
		return false
	}
	_, has := attrs[auth_attributes_name]
	return has
}