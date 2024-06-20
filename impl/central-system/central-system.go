package centralsystem

import (
	"bytes"
	"encoding/json"
	"evsys-back/entity"
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

func (cs *CentralSystem) SendCommand(command *entity.CentralSystemCommand) *entity.CentralSystemResponse {
	response := entity.NewCentralSystemResponse(command.ChargePointId, command.ConnectorId)

	data, err := json.Marshal(command)
	if err != nil {
		response.SetError(fmt.Sprintf("marshalling command %s: %v", command.FeatureName, err))
		return response
	}

	req, err := http.NewRequest("POST", cs.url, bytes.NewBuffer(data))
	if err != nil {
		response.SetError(fmt.Sprintf("creating request: %v", err))
		return response
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cs.token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		response.SetError(fmt.Sprintf("sending command %s: %v", command.FeatureName, err))
		return response
	}
	if resp == nil {
		response.SetError(fmt.Sprintf("sending command %s: no response", command.FeatureName))
		return response
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("closing response body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		response.SetError(fmt.Sprintf("sending command %s: response status %v", command.FeatureName, resp.StatusCode))
		return response
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		response.SetError(fmt.Sprintf("reading response body: %v", err))
		return response
	}
	response.Info = string(bodyBytes)
	return response
}
