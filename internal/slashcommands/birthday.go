package slashcommands

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/zekroTJA/shinpuru/internal/models"
	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/services/permissions"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekrotja/ken"
)

var (
	dateRe = regexp.MustCompile(`^(?:(\d{4})[\/\-\.])?(\d{1,2})[\/\-\.](\d{1,2})([\+\-](?:\d{1,2}))?$`)

	errYear = errors.New("you need to specify a year when you want to show your birthday year")
)

type Birthday struct {
	ken.EphemeralCommand
}

var (
	_ ken.SlashCommand        = (*Birthday)(nil)
	_ permissions.PermCommand = (*Birthday)(nil)
)

func (c *Birthday) Name() string {
	return "birthday"
}

func (c *Birthday) Description() string {
	return "Set or manage birthdays."
}

func (c *Birthday) Version() string {
	return "1.0.0"
}

func (c *Birthday) Type() discordgo.ApplicationCommandType {
	return discordgo.ChatApplicationCommand
}

func (c *Birthday) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "set-channel",
			Description: "Set birthday message channel.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type: discordgo.ApplicationCommandOptionChannel,
					Name: "channel",
					ChannelTypes: []discordgo.ChannelType{
						discordgo.ChannelTypeGuildText,
					},
					Description: "The birthday message channel.",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "unset-channel",
			Description: "Unset birthday message channel.",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "set",
			Description: "Set your birthday.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "date",
					Description: "The birthday date in format (YYYY-MM-DD+TZ).",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "show-year",
					Description: "Whether or not to show the year in the birtday message.",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "remove",
			Description: "Remove your birthday.",
		},
	}
}

func (c *Birthday) Domain() string {
	return "sp.chat.birthday"
}

func (c *Birthday) SubDomains() []permissions.SubPermission {
	return []permissions.SubPermission{
		{
			Term:        "/sp.guild.config.birthday",
			Explicit:    false,
			Description: "Allows setting the birthday channel.",
		},
	}
}

func (c *Birthday) Run(ctx ken.Context) (err error) {
	if err = ctx.Defer(); err != nil {
		return
	}

	err = ctx.HandleSubCommands(
		ken.SubCommandHandler{"set-channel", c.setChannel},
		ken.SubCommandHandler{"unset-channel", c.unsetChannel},
		ken.SubCommandHandler{"set", c.set},
		ken.SubCommandHandler{"remove", c.remove},
	)

	return
}

func (c *Birthday) setChannel(ctx ken.SubCommandContext) (err error) {
	db := ctx.Get(static.DiDatabase).(database.Database)
	pmw := ctx.Get(static.DiPermissions).(*permissions.Permissions)

	ok, err := pmw.CheckSubPerm(ctx, "/sp.guild.settings.birthday", false,
		"You are not permitted to edit the guild birthday channel.")
	if !ok {
		return
	}

	ch := ctx.Options().GetByName("channel").ChannelValue(ctx)
	err = db.SetGuildBirthdayChan(ctx.GetEvent().GuildID, ch.ID)
	if err != nil {
		return
	}

	err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: fmt.Sprintf(
			"Birthday channel has been set to <#%s>.",
			ch.ID),
	}).Send().Error

	return
}

func (c *Birthday) unsetChannel(ctx ken.SubCommandContext) (err error) {
	db := ctx.Get(static.DiDatabase).(database.Database)
	pmw := ctx.Get(static.DiPermissions).(*permissions.Permissions)

	ok, err := pmw.CheckSubPerm(ctx, "/sp.guild.settings.birthday", false,
		"You are not permitted to edit the guild birthday channel.")
	if !ok {
		return
	}

	err = db.SetGuildBirthdayChan(ctx.GetEvent().GuildID, "")
	if err != nil {
		return
	}

	err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: "Birthday channel has been reset.",
	}).Send().Error

	return
}

func (c *Birthday) set(ctx ken.SubCommandContext) (err error) {
	db := ctx.Get(static.DiDatabase).(database.Database)

	dateStr := ctx.Options().GetByName("date").StringValue()
	showYear := ctx.Options().GetByName("show-year").BoolValue()

	matches := dateRe.FindAllStringSubmatch(dateStr, -1)
	if len(matches) == 0 || len(matches[0]) != 5 {
		return ctx.FollowUpError(
			"Invalid date format.\n\n"+
				"The expected date format is `YYYY-MM-DD` or `MM-DD`. "+
				"You can also use `/` or `.` as delimiters.\n\n"+
				"You might also want to attach a timezone offset in the "+
				"format of `[+-]TZ` (in hours offset of UTC).", "").
			Send().
			Error
	}
	date, err := parseDate(matches[0], showYear)
	if err == errYear {
		err = ctx.FollowUpError(err.Error(), "").Send().Error
		return
	}
	if err != nil {
		return
	}

	err = db.SetBirthday(models.Birthday{
		GuildID:  ctx.GetEvent().GuildID,
		UserID:   ctx.User().ID,
		Date:     date,
		ShowYear: showYear,
	})
	if err != nil {
		return
	}

	emb := &discordgo.MessageEmbed{
		Description: "Your birthday has successfully been registered.",
	}
	err = ctx.FollowUpEmbed(emb).Send().Error

	return
}

func (c *Birthday) remove(ctx ken.SubCommandContext) (err error) {
	db := ctx.Get(static.DiDatabase).(database.Database)

	err = db.DeleteBirthday(ctx.GetEvent().GuildID, ctx.User().ID)
	if err != nil {
		return
	}

	err = ctx.RespondEmbed(&discordgo.MessageEmbed{
		Description: "Your birthday has successfully been unregistered.",
	})

	return
}

func parseDate(matches []string, showYear bool) (date time.Time, err error) {
	var y, m, d, offset = 1970, 0, 0, 0
	if matches[1] != "" {
		if y, err = strconv.Atoi(matches[1]); err != nil {
			return
		}
	} else if showYear {
		err = errYear
		return
	}
	if m, err = strconv.Atoi(matches[2]); err != nil {
		return
	}
	if d, err = strconv.Atoi(matches[3]); err != nil {
		return
	}
	if offsetStr := matches[4]; offsetStr != "" {
		prefix := offsetStr[0]
		if offset, err = strconv.Atoi(offsetStr[1:]); err != nil {
			return
		}
		if offset < 0 || offset > 23 {
			err = errors.New("timezone offset must be in range [-23, 23]")
			return
		}
		if prefix == '-' {
			offset = 24 - offset
		}
	}
	date = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.FixedZone("Offset", offset*3600))
	return
}
