package services

import "evsys-back/models"

type CentralSystemService interface {
	SendCommand(command *models.CentralSystemCommand) (string, error)
}
