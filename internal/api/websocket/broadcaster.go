package websocket

import (
	"context"
	"encoding/json"
	"evsys-back/entity"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"time"
)

// Broadcaster listens for log updates and distributes events to subscribed clients
type Broadcaster struct {
	pool   *Pool
	sr     StatusReader
	logger *slog.Logger
}

// NewBroadcaster creates a new Broadcaster instance
func NewBroadcaster(pool *Pool, sr StatusReader, logger *slog.Logger) *Broadcaster {
	return &Broadcaster{
		pool:   pool,
		sr:     sr,
		logger: logger,
	}
}

// Start begins listening for updates and broadcasting them to subscribed clients
func (b *Broadcaster) Start(ctx context.Context) {
	if b.sr == nil {
		// No status reader configured, nothing to broadcast
		<-ctx.Done()
		return
	}

	lastMessageTime := time.Now()
	waitStep := 5
	ticker := time.NewTicker(time.Duration(waitStep) * time.Second)

	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			messages, _ := b.sr.ReadLogAfter(reqCtx, lastMessageTime)
			cancel()
			if messages == nil {
				continue
			}
			if len(messages) > 0 {
				lastMessageTime = messages[len(messages)-1].Timestamp
				for _, message := range messages {

					if len(message.ChargePointId) > 1 {
						b.pool.SendChpEvent(&entity.WsResponse{
							Status: entity.Event,
							Stage:  entity.ChargePointEvent,
							Data:   message.ChargePointId,
							Info:   message.Text,
						})
					}

					data, err := json.Marshal(message)
					if err != nil {
						b.logger.Error("marshal log message", sl.Err(err))
						continue
					}
					b.pool.SendLogEvent(&entity.WsResponse{
						Status: entity.Event,
						Stage:  entity.LogEvent,
						Data:   string(data),
						Info:   message.Text,
					})

				}
			}
		}
	}
}
