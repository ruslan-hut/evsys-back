package core

import "evsys-back/entity"

type CentralSystem interface {
	SendCommand(command *entity.CentralSystemCommand) *entity.CentralSystemResponse
}
