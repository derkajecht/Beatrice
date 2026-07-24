package errors

import (
	"log/slog"

	"github.com/derkajecht/Beatrice/internal/models"
	"github.com/derkajecht/Beatrice/internal/types"
	"github.com/derkajecht/Beatrice/internal/utils"
)

func SendErrorPacket(errMsg string, c *types.Client, slogMsg, errType string) error {
	errPacket := models.NewErrPacket(errMsg)
	err := utils.SendPacketToClient(c.Conn, "e", errPacket)

	slog.Error(slogMsg, "err", errType, "client", c.Conn.RemoteAddr())
	return err
}
