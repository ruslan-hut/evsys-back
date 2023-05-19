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

func (cs *CentralSystem) SendCommand(command *models.CentralSystemCommand) error {
	log.Printf("* SendCommand: %v", command)
	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("error marshalling command: %v", err)
	}

	req, err := http.NewRequest("POST", cs.url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
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
		return fmt.Errorf("error sending command: %v; response status: %v", err, resp.StatusCode)
	}

	return nil
}
