package services

import "evsys-back/entity"

type CentralSystemService interface {
	SendCommand(command *entity.CentralSystemCommand) (string, error)
}
