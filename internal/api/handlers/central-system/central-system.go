package centralsystem

import (
	"evsys-back/entity"
	"evsys-back/internal/lib/api/cont"
	"evsys-back/internal/lib/api/web"
	"evsys-back/internal/lib/sl"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
)

type CentralSystem interface {
	SendCommand(command *entity.CentralSystemCommand, user *entity.User) (any, error)
}

func Command(logger *slog.Logger, handler CentralSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := cont.GetUser(ctx)
		log := web.Log(ctx, logger, "handlers.central_system",
			slog.String("user", user.Username),
			slog.String("role", user.Role),
			slog.Int("access_level", user.AccessLevel),
		)

		var command entity.CentralSystemCommand
		if err := render.Bind(r, &command); err != nil {
			web.Fail(w, r, log, 400, "Failed to decode", err)
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
			web.Fail(w, r, log, 400, "Failed to send command", err)
			return
		}
		web.OK(w, r, log, "cs command success", data)
	}
}
