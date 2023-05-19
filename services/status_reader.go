package services

import (
	"evsys-back/models"
	"time"
)

type StatusReader interface {
	GetTransaction(userId string, after time.Time) (*models.Transaction, error)
}
