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
	url   string
	token string
}

func NewCentralSystem(url, token string) *CentralSystem {
	return &CentralSystem{url: url, token: token}
}

func (cs *CentralSystem) SendCommand(command *models.CentralSystemCommand) (string, error) {
	log.Printf("* SendCommand: %v", command)
	data, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("marshalling command: %v", err)
	}

	req, err := http.NewRequest("POST", cs.url, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cs.token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending command %s: %v", command.FeatureName, err)
	}
	if resp == nil {
		return "", fmt.Errorf("sending command %s: no response", command.FeatureName)
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sending command %s: response status %v", command.FeatureName, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %v", err)
	}

	return string(bodyBytes), nil
}
