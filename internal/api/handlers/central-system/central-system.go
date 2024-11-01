package centralsystem

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/response"
	"evsys-back/internal/lib/sl"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
)

type CentralSystem interface {
	SendCommand(command *entity.CentralSystemCommand, user *entity.User) (interface{}, error)
}

func Command(logger *slog.Logger, handler CentralSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		user := cont.GetUser(r.Context())

		log := logger.With(
			sl.Module("handlers.central_system"),
			slog.String("user", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var command entity.CentralSystemCommand
		if err := render.Bind(r, &command); err != nil {
			log.With(sl.Err(err)).Error("bind failed")
			render.Status(r, 400)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to decode: %v", err)))
			return
		}
		log = log.With(
			slog.String("charge_point_id", command.ChargePointId),
			slog.Int("connector_id", command.ConnectorId),
			slog.String("feature_name", command.FeatureName),
			sl.Secret("payload", command.Payload),
		)

		data, err := handler.SendCommand(&command, user)
		if err != nil {
			log.With(sl.Err(err)).Error("send cs command failed")
			render.Status(r, 204)
			render.JSON(w, r, response.Error(2001, fmt.Sprintf("Failed to send command: %v", err)))
			return
		}
		log.Info("cs command success", slog.Any("data", data))

		render.JSON(w, r, data)
	}
}
