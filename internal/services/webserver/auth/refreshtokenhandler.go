package auth

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	"github.com/zekroTJA/shinpuru/internal/services/timeprovider"
	"github.com/zekroTJA/shinpuru/internal/services/webserver/v1/models"
	"github.com/zekroTJA/shinpuru/internal/util/embedded"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekroTJA/shinpuru/pkg/discordoauth/v2"
)

var _ RequestHandler = (*RefreshTokenRequestHandler)(nil)

// RefreshTokenRequestHandler implements RequestHandler for
// the refresh-access token authentication
// system.
type RefreshTokenRequestHandler struct {
	session             Session
	tp                  TimeProvider
	accessTokenHandler  AccessTokenHandler
	refreshTokenHandler RefreshTokenHandler
}

func NewRefreshTokenRequestHandler(container di.Container) *RefreshTokenRequestHandler {
	return &RefreshTokenRequestHandler{
		session:             container.Get(static.DiDiscordSession).(*discordgo.Session),
		accessTokenHandler:  container.Get(static.DiAuthAccessTokenHandler).(AccessTokenHandler),
		refreshTokenHandler: container.Get(static.DiAuthRefreshTokenHandler).(RefreshTokenHandler),
		tp:                  container.Get(static.DiTimeProvider).(timeprovider.Provider),
	}
}

func (h *RefreshTokenRequestHandler) LoginFailedHandler(ctx *fiber.Ctx, status int, msg string) error {
	return fiber.NewError(status, msg)
}

func (h *RefreshTokenRequestHandler) BindRefreshToken(ctx *fiber.Ctx, uid string) error {
	user, _ := h.session.User(uid)
	if user == nil {
		return fiber.ErrUnauthorized
	}

	ctx.Locals("uid", uid)
	refreshToken, err := h.refreshTokenHandler.GetRefreshToken(uid)
	if err != nil {
		return err
	}

	expires := h.tp.Now().Add(static.AuthSessionExpiration)
	ctx.Cookie(&fiber.Cookie{
		Name:     static.RefreshTokenCookieName,
		Value:    refreshToken,
		Path:     "/",
		Expires:  expires,
		HTTPOnly: true,
		Secure:   embedded.IsRelease(),
	})

	return nil
}

func (h *RefreshTokenRequestHandler) LoginSuccessHandler(ctx *fiber.Ctx, res discordoauth.SuccessResult) error {
	if err := h.BindRefreshToken(ctx, res.UserID); err != nil {
		return err
	}

	location := "/"
	if redirect, ok := res.State["redirect"]; ok {
		location += strings.TrimLeft(redirect, "/")
	}

	return ctx.Redirect(location, fiber.StatusTemporaryRedirect)
}

func (h *RefreshTokenRequestHandler) LogoutHandler(ctx *fiber.Ctx) error {
	if uid, ok := ctx.Locals("uid").(string); ok && uid != "" {
		if err := h.refreshTokenHandler.RevokeToken(uid); err != nil {
			return err
		}
	}

	ctx.ClearCookie(static.RefreshTokenCookieName)

	return ctx.JSON(models.Ok)
}
