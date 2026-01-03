package websocket

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"fmt"
	"time"
)

func (c *Client) listenForTransactionStart(timeStart time.Time) {

	maxTimeout := 90
	waitStep := 2

	duration := maxTimeout - int(time.Since(timeStart).Seconds())
	if duration <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)
	pause := time.NewTimer(time.Duration(duration) * time.Second)

	defer func() {
		ticker.Stop()
		pause.Stop()
		if !c.isClosed {
			c.statusReader.ClearStatus(c.id)
		}
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			transaction, err := c.statusReader.GetTransactionAfter(ctx, c.id, timeStart)
			cancel()
			if err != nil {
				c.logger.Error("get transaction", sl.Err(err))
				continue
			}
			if transaction.TransactionId > -1 {
				c.WsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Start,
					Id:     transaction.TransactionId,
					Info:   fmt.Sprintf("transaction started: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.WsResponse(&entity.WsResponse{
					Status:   entity.Waiting,
					Stage:    entity.Start,
					Id:       -1,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			c.WsResponse(&entity.WsResponse{
				Status: entity.Error,
				Stage:  entity.Start,
				Info:   "timeout",
			})
			return
		}
	}
}

func (c *Client) listenForTransactionStop(timeStart time.Time, transactionId int) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := c.statusReader.GetTransaction(ctx, transactionId)
	cancel()
	if err != nil {
		c.WsResponse(&entity.WsResponse{
			Status: entity.Error,
			Stage:  entity.Stop,
			Info:   fmt.Sprintf("%v", err),
		})
		return
	}

	maxTimeout := 90
	waitStep := 3

	duration := maxTimeout - int(time.Since(timeStart).Seconds())
	if duration <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)
	pause := time.NewTimer(time.Duration(duration) * time.Second)

	defer func() {
		ticker.Stop()
		pause.Stop()
		if !c.isClosed {
			c.statusReader.ClearStatus(c.id)
		}
	}()

	for {
		select {
		case <-ticker.C:
			if c.isClosed {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			transaction, err := c.statusReader.GetTransaction(ctx, transactionId)
			cancel()
			if err != nil {
				c.logger.Error("get transaction", sl.Err(err))
				continue
			}
			if transaction.IsFinished {
				c.WsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Stop,
					Id:     transaction.TransactionId,
					Info:   fmt.Sprintf("transaction stopped: %v", transaction.TransactionId),
				})
				return
			} else {
				seconds := int(time.Since(timeStart).Seconds())
				progress := seconds * 100 / maxTimeout
				c.WsResponse(&entity.WsResponse{
					Status:   entity.Waiting,
					Stage:    entity.Stop,
					Id:       transaction.TransactionId,
					Info:     fmt.Sprintf("waiting %vs; %v%%", seconds, progress),
					Progress: progress,
				})
			}
		case <-pause.C:
			c.WsResponse(&entity.WsResponse{
				Status: entity.Error,
				Stage:  entity.Stop,
				Info:   "timeout",
			})
			return
		}
	}
}

func (c *Client) listenForTransactionState(transactionId int) {
	if transactionId < 0 {
		return
	}

	lastMeterValue := time.Now()
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for range ticker.C {
		if c.isClosed {
			return
		}
		_, ok := c.listeners[transactionId]
		if !ok {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		values, _ := c.statusReader.GetLastMeterValues(ctx, transactionId, lastMeterValue)
		cancel()
		if values == nil {
			continue
		}
		for i := range values {
			values[i].Timestamp = values[i].Time.Unix()
			c.WsResponse(&entity.WsResponse{
				Status:          entity.Value,
				Stage:           entity.Info,
				Info:            values[i].Measurand,
				Power:           values[i].ConsumedEnergy,
				PowerRate:       values[i].PowerRate,
				SoC:             values[i].BatteryLevel,
				Price:           values[i].Price,
				Minute:          values[i].Minute,
				Id:              transactionId,
				ConnectorId:     values[i].ConnectorId,
				ConnectorStatus: values[i].ConnectorStatus,
				MeterValue:      &values[i],
			})
			lastMeterValue = values[i].Time
			time.Sleep(1 * time.Second)
		}
	}
}

func (c *Client) listenForLogUpdates() {

	lastMessageTime := time.Now()
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for range ticker.C {
		if c.isClosed {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		messages, _ := c.statusReader.ReadLogAfter(ctx, lastMessageTime)
		cancel()
		if messages == nil {
			continue
		}
		if len(messages) > 0 {
			lastMessageTime = messages[len(messages)-1].Timestamp
			for _, message := range messages {
				data, err := json.Marshal(message)
				if err != nil {
					c.logger.Error("marshal message", sl.Err(err))
					continue
				}
				c.WsResponse(&entity.WsResponse{
					Status: entity.Success,
					Stage:  entity.Info,
					Data:   string(data),
				})
			}
		}
	}
}
