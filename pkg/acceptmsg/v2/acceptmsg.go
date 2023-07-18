// Package acceptmsg provides a message model for
// discordgo which can be accepted or declined
// via message reactions.
package acceptmsg

import (
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/xid"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekrotja/ken"
)

var (
	ErrTimeout = errors.New("timed out")

	acceptButton = discordgo.Button{
		Label: "Accept",
		Style: discordgo.SuccessButton,
	}

	declineButton = discordgo.Button{
		Label: "Decline",
		Style: discordgo.DangerButton,
	}
)

type ActionHandler func(ctx ken.ComponentContext) error

// AcceptMessage extends discordgo.Message to build
// and send an AcceptMessage.
type AcceptMessage struct {
	*discordgo.Message

	Ken            *ken.Ken
	Embed          *discordgo.MessageEmbed
	UserID         string
	DeleteMsgAfter bool
	AcceptButton   *discordgo.Button
	DeclineButton  *discordgo.Button
	AcceptFunc     ActionHandler
	DeclineFunc    ActionHandler

	mtx       sync.Mutex
	cErr      chan error
	timeout   time.Duration
	activated bool
	unreg     func() error
}

// New creates an empty instance of AcceptMessage.
func New() *AcceptMessage {
	return new(AcceptMessage)
}

// WithKen sets the ken instance.
func (am *AcceptMessage) WithKen(ken *ken.Ken) *AcceptMessage {
	am.Ken = ken
	return am
}

// WithEmbed sets the Embed instance to be set.
func (am *AcceptMessage) WithEmbed(e *discordgo.MessageEmbed) *AcceptMessage {
	am.Embed = e
	return am
}

// WithContent creates an embed with default color and
// specified content as description and sets it as
// embed instance.
func (am *AcceptMessage) WithContent(cont string) *AcceptMessage {
	am.Embed = &discordgo.MessageEmbed{
		Color:       static.ColorEmbedDefault,
		Description: cont,
	}
	return am
}

// LockOnUser specifies, that only reaction inputs from
// the defined user are accepted.
func (am *AcceptMessage) LockOnUser(userID string) *AcceptMessage {
	am.UserID = userID
	return am
}

// DeleteAfterAnswer enables that the whole accept
// embed message is being deleted after users
// answer.
func (am *AcceptMessage) DeleteAfterAnswer() *AcceptMessage {
	am.DeleteMsgAfter = true
	return am
}

// DoOnAccept specifies the action handler executed
// on acception.
func (am *AcceptMessage) DoOnAccept(onAccept ActionHandler) *AcceptMessage {
	am.AcceptFunc = onAccept
	return am
}

// DoOnDecline specifies the action handler executed
// on decline.
func (am *AcceptMessage) DoOnDecline(onDecline ActionHandler) *AcceptMessage {
	am.DeclineFunc = onDecline
	return am
}

func (am *AcceptMessage) WithTimeout(t time.Duration) *AcceptMessage {
	am.timeout = t
	return am
}

// Error blocks until either one of the action functions was
// called or until the accept message timed out. Then, it
// returns an error or nil.
func (am *AcceptMessage) Error() error {
	return <-am.cErr
}

func (am *AcceptMessage) WithAcceptButton(btn discordgo.Button) *AcceptMessage {
	am.AcceptButton = &btn
	return am
}

func (am *AcceptMessage) WithDeclineButton(btn discordgo.Button) *AcceptMessage {
	am.DeclineButton = &btn
	return am
}

func (am *AcceptMessage) attachComponents(cb *ken.ComponentBuilder) error {
	if am.Ken == nil {
		return errors.New("ken is not defined")
	}
	if am.Embed == nil {
		return errors.New("embed not defined")
	}

	if am.timeout <= 0 {
		am.timeout = 1 * time.Minute
	}

	if am.AcceptFunc == nil {
		am.AcceptFunc = func(ken.ComponentContext) error {
			return nil
		}
	}

	if am.DeclineFunc == nil {
		am.DeclineFunc = func(ken.ComponentContext) error {
			return nil
		}
	}

	if am.AcceptButton == nil {
		am.AcceptButton = &acceptButton
	}
	if am.DeclineButton == nil {
		am.DeclineButton = &declineButton
	}

	am.cErr = make(chan error, 1)

	wrapHandler := func(h ActionHandler) func(ctx ken.ComponentContext) bool {
		return func(ctx ken.ComponentContext) bool {
			var userId string
			if ctx.GetEvent().User != nil {
				userId = ctx.GetEvent().User.ID
			} else if ctx.GetEvent().Member != nil {
				userId = ctx.GetEvent().Member.User.ID
			}

			am.mtx.Lock()
			defer am.mtx.Unlock()

			if am.UserID != "" && userId != am.UserID {
				return false
			}
			err := h(ctx)
			if err != nil {
				am.cErr <- err
			}
			am.activated = true

			if am.DeleteMsgAfter {
				time.AfterFunc(1*time.Second, func() {
					am.Ken.Session().ChannelMessageDelete(am.ChannelID, am.ID)
				})
			}

			return true
		}
	}

	id := xid.New().String()
	am.AcceptButton.CustomID = id + "-" + "accept"
	am.DeclineButton.CustomID = id + "-" + "decline"

	cb.AddActionsRow(func(b ken.ComponentAssembler) {
		b.Add(*am.AcceptButton, wrapHandler(am.AcceptFunc))
		b.Add(*am.DeclineButton, wrapHandler(am.DeclineFunc))
	}, true)

	go func() {
		time.Sleep(am.timeout)

		am.mtx.Lock()
		defer am.mtx.Unlock()

		if am.activated {
			return
		}
		am.cErr <- ErrTimeout
		if am.unreg != nil {
			am.unreg()
		}
	}()

	return nil
}

// Send pushes the accept message into the specified
// channel and sets up listener handlers for reactions.
func (am *AcceptMessage) Send(chanID string) (*AcceptMessage, error) {
	msg, err := am.Ken.Session().ChannelMessageSendEmbed(chanID, am.Embed)
	if err != nil {
		return nil, err
	}

	cb := am.Ken.Components().Add(msg.ID, msg.ChannelID)
	am.attachComponents(cb)
	unreg, err := cb.Build()
	if err != nil {
		return nil, err
	}

	am.Message = msg
	am.unreg = unreg
	return am, nil
}

// AsFollowUp pushes the accept messages as follow up
// message to the command context and sets up listener
// handlers for reactions.
func (am *AcceptMessage) AsFollowUp(ctx ken.Context) (*AcceptMessage, error) {
	fum := ctx.FollowUpEmbed(am.Embed).AddComponents(func(cb *ken.ComponentBuilder) {
		am.attachComponents(cb)
	}).Send()
	if fum.HasError() {
		return nil, fum.Error
	}

	am.unreg = fum.UnregisterComponentHandlers
	am.Message = fum.Message
	return am, nil
}
