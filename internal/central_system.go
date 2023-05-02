package internal

import (
	"bytes"
	"encoding/json"
	"evsys-back/models"
	"fmt"
	"io"
	"log"
	"net/http"
)

type CentralSystem struct {
	url string
}

func NewCentralSystem(url string) *CentralSystem {
	return &CentralSystem{url: url}
}

func (cs *CentralSystem) SendCommand(command *models.CentralSystemCommand) (*models.CentralSystemResponse, error) {
	log.Printf("SendCommand: %v", command)
	data, err := json.Marshal(command)
	if err != nil {
		return models.NewCentralSystemResponse(models.Error, "invalid data"), err
	}

	req, err := http.NewRequest("POST", cs.url, bytes.NewBuffer(data))
	if err != nil {
		return models.NewCentralSystemResponse(models.Error, "invalid request"), err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if resp != nil {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Printf("error closing response body: %v", err)
			}
		}(resp.Body)
	}

	if err != nil || resp.StatusCode != http.StatusOK {
		return models.NewCentralSystemResponse(models.Error, fmt.Sprintf("error sending command; %v", resp.Status)), err
	}

	return models.NewCentralSystemResponse(models.Success, ""), nil
}
