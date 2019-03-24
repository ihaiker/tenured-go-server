package store

import (
	"github.com/ihaiker/tenured-go-server/api"
	"github.com/ihaiker/tenured-go-server/api/command"
	"github.com/ihaiker/tenured-go-server/commons/executors"
	"github.com/ihaiker/tenured-go-server/commons/protocol"
)

type AccountServer struct {
}

func (this *AccountServer) Apply(account *command.Account) (*command.Account, *protocol.TenuredError) {
	account.Name = "NewName"
	return account, nil
}

func handlerAccountServer(server *protocol.TenuredServer) (accountServer *AccountServer, err error) {
	accountServer = &AccountServer{}
	invoke := protocol.NewInvoke(server, accountServer)
	executor := executors.NewFixedExecutorService(10, 1000)
	if err = invoke.Invoke(api.AccountServiceApply, "Apply", executor); err != nil {
		return
	}
	return
}