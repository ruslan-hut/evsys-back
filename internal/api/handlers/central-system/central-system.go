package centralsystem

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
)

type CentralSystem interface {
	SendCommand(command *entity.CentralSystemCommand) (interface{}, error)
}

func Command(logger *slog.Logger, handler CentralSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		log := logger.With(
			sl.Module("handlers.central_system"),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var command entity.CentralSystemCommand
		if err := render.Bind(r, &command); err != nil {
			log.Error("bind", sl.Err(err))
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("charge_point_id", command.ChargePointId),
			slog.Int("connector_id", command.ConnectorId),
			slog.String("feature_name", command.FeatureName),
			slog.String("payload", command.Payload),
		)

		data, err := handler.SendCommand(&command)
		if err != nil {
			log.Error("cs command", sl.Err(err))
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to send command: %v", err)))
			return
		}
		log.Info("cs command success", slog.Any("data", data))

		render.JSON(w, r, data)
	}
}
