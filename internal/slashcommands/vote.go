package slashcommands

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/services/permissions"
	"github.com/zekroTJA/shinpuru/internal/services/timeprovider"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekroTJA/shinpuru/internal/util/vote"
	"github.com/zekroTJA/shinpuru/pkg/discordutil"
	"github.com/zekroTJA/shinpuru/pkg/timeutil"
	"github.com/zekrotja/ken"
)

type Vote struct{}

var (
	_ ken.SlashCommand        = (*Vote)(nil)
	_ permissions.PermCommand = (*Vote)(nil)
)

func (c *Vote) Name() string {
	return "vote"
}

func (c *Vote) Description() string {
	return "Create and manage votes."
}

func (c *Vote) Version() string {
	return "1.0.0"
}

func (c *Vote) Type() discordgo.ApplicationCommandType {
	return discordgo.ChatApplicationCommand
}

func (c *Vote) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "create",
			Description: "Create a new vote.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "body",
					Description: "The vote body content.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "choises",
					Description: "The choises - split by `,`.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "imageurl",
					Description: "An optional image URL.",
				},
				{
					Type:         discordgo.ApplicationCommandOptionChannel,
					Name:         "channel",
					Description:  "The channel to create the vote in (defaultly the current channel).",
					ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "timeout",
					Description: "Timeout of the vote (i.e. `1h`, `30m`, ...)",
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "list",
			Description: "List currently running votes.",
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "expire",
			Description: "Set the expiration of a running vote.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "The ID of the vote or `all` if you want to close all.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "timeout",
					Description: "Timeout of the vote (i.e. `1h`, `30m`, ...)",
					Required:    true,
				},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "close",
			Description: "Close a running vote.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "The ID of the vote or `all` if you want to close all.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "chart",
					Description: "Display chart (default `true`).",
				},
			},
		},
	}
}

func (c *Vote) Domain() string {
	return "sp.chat.vote"
}

func (c *Vote) SubDomains() []permissions.SubPermission {
	return []permissions.SubPermission{
		{
			Term:        "close",
			Explicit:    true,
			Description: "Allows closing votes also from other users",
		},
	}
}

func (c *Vote) Run(ctx ken.Context) (err error) {
	if err = ctx.Defer(); err != nil {
		return
	}

	err = ctx.HandleSubCommands(
		ken.SubCommandHandler{"create", c.create},
		ken.SubCommandHandler{"list", c.list},
		ken.SubCommandHandler{"expire", c.expire},
		ken.SubCommandHandler{"close", c.close},
	)

	return
}

func (c *Vote) create(ctx ken.SubCommandContext) (err error) {
	db, _ := ctx.Get(static.DiDatabase).(database.Database)
	tp, _ := ctx.Get(static.DiTimeProvider).(timeprovider.Provider)

	body := ctx.Options().GetByName("body").StringValue()
	choises := ctx.Options().GetByName("choises").StringValue()
	split := strings.Split(choises, ",")
	if len(split) < 2 || len(split) > 10 {
		return ctx.FollowUpError(
			"Invalid arguments. Please use `help vote` go get help about how to use this command.", "").
			Send().Error
	}
	for i, e := range split {
		if len(e) < 1 {
			return ctx.FollowUpError(
				"Possibilities can not be empty.", "").
				Send().Error
		}
		split[i] = strings.Trim(e, " \t")
	}

	var imgLink string
	if imgLinkV, ok := ctx.Options().GetByNameOptional("imageurl"); ok {
		imgLink = imgLinkV.StringValue()
	}

	var expires time.Time
	if expiresV, ok := ctx.Options().GetByNameOptional("timeout"); ok {
		expiresDuration, err := timeutil.ParseDuration(expiresV.StringValue())
		if err != nil {
			return ctx.FollowUpError(
				"Invalid duration format. Please take a look "+
					"[here](https://golang.org/pkg/time/#ParseDuration) how to format duration parameter.", "").
				Send().Error
		}
		expires = tp.Now().Add(expiresDuration)
	}

	ivote := vote.Vote{
		ID:            ctx.GetEvent().ID,
		MsgID:         "",
		CreatorID:     ctx.User().ID,
		GuildID:       ctx.GetEvent().GuildID,
		ChannelID:     ctx.GetEvent().ChannelID,
		Description:   body,
		Possibilities: split,
		ImageURL:      imgLink,
		Expires:       expires,
		Ticks:         make(map[string]*vote.Tick),
	}

	emb, err := ivote.AsEmbed(ctx.GetSession())
	if err != nil {
		return err
	}

	chV, ok := ctx.Options().GetByNameOptional("channel")

	var msg *discordgo.Message
	if ok {
		ch := chV.ChannelValue(ctx)
		msg, err = ctx.GetSession().ChannelMessageSendEmbed(ch.ID, emb)
		if err != nil {
			return
		}
		msgLink := discordutil.GetMessageLink(msg, ctx.GetEvent().GuildID)
		err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
			Description: fmt.Sprintf("[Vote](%s) created in channel <#%s>.", msgLink, ch.ID),
		}).Send().Error
		if err != nil {
			return
		}
	} else {
		fum := ctx.FollowUpEmbed(emb).Send()
		err = fum.Error
		if err != nil {
			return
		}
		msg = fum.Message
	}

	ivote.MsgID = msg.ID
	err = ivote.AddReactions(ctx.GetSession())
	if err != nil {
		return err
	}

	err = db.AddUpdateVote(ivote)
	if err != nil {
		return err
	}

	vote.VotesRunning[ivote.ID] = ivote
	return
}

func (c *Vote) list(ctx ken.SubCommandContext) (err error) {
	emb := &discordgo.MessageEmbed{
		Description: "Your open votes on this guild:",
		Color:       static.ColorEmbedDefault,
		Fields:      make([]*discordgo.MessageEmbedField, 0),
	}
	for _, v := range vote.VotesRunning {
		if v.GuildID == ctx.GetEvent().GuildID && v.CreatorID == ctx.User().ID {
			emb.Fields = append(emb.Fields, v.AsField())
		}
	}
	if len(emb.Fields) == 0 {
		emb.Description = "You don't have any open votes on this guild."
	}
	err = ctx.FollowUpEmbed(emb).Send().Error
	return err
}

func (c *Vote) expire(ctx ken.SubCommandContext) (err error) {
	db, _ := ctx.Get(static.DiDatabase).(database.Database)

	expireDuration, err := timeutil.ParseDuration(ctx.Options().GetByName("timeout").StringValue())
	if err != nil {
		return ctx.FollowUpError(
			"Invalid duration format. Please take a look "+
				"[here](https://golang.org/pkg/time/#ParseDuration) how to format duration parameter.", "").
			Send().Error
	}

	id := ctx.Options().Get(0).StringValue()
	var ivote *vote.Vote
	for _, v := range vote.VotesRunning {
		if v.GuildID == ctx.GetEvent().GuildID && v.ID == id {
			ivote = &v
		}
	}

	tp := ctx.Get(static.DiTimeProvider).(timeprovider.Provider)

	ivote.SetExpire(ctx.GetSession(), expireDuration, tp)
	if err = db.AddUpdateVote(*ivote); err != nil {
		return err
	}

	return ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: fmt.Sprintf("Vote will expire at %s.", ivote.Expires.Format("01/02 15:04 MST")),
	}).Send().Error
}

func (c *Vote) close(ctx ken.SubCommandContext) (err error) {
	db, _ := ctx.Get(static.DiDatabase).(database.Database)

	state := vote.VoteStateClosed

	if showChartV, ok := ctx.Options().GetByNameOptional("chart"); ok && !showChartV.BoolValue() {
		state = vote.VoteStateClosedNC
	}

	id := ctx.Options().GetByName("id").StringValue()

	if strings.ToLower(id) == "all" {
		var i int
		for _, v := range vote.VotesRunning {
			if v.GuildID == ctx.GetEvent().GuildID && v.CreatorID == ctx.User().ID {
				go func(vC vote.Vote) {
					db.DeleteVote(vC.ID)
					vC.Close(ctx.GetSession(), state)
				}(v)
				i++
			}
		}
		return ctx.FollowUpEmbed(&discordgo.MessageEmbed{
			Description: fmt.Sprintf("Closed %d votes.", i),
		}).Send().Error
	}

	var ivote *vote.Vote
	for _, v := range vote.VotesRunning {
		if v.GuildID == ctx.GetEvent().GuildID && v.ID == id {
			ivote = &v
			break
		}
	}

	pmw, _ := ctx.Get(static.DiPermissions).(*permissions.Permissions)
	ok, override, err := pmw.CheckPermissions(ctx.GetSession(), ctx.GetEvent().GuildID, ctx.User().ID, "!"+ctx.GetCommand().(permissions.PermCommand).Domain()+".close")
	if err != nil {
		return err
	}
	if ivote.CreatorID != ctx.User().ID && !ok && !override {
		return ctx.FollowUpError(
			"You do not have the permission to close another ones votes.", "").
			Send().Error
	}

	err = db.DeleteVote(ivote.ID)
	if err != nil {
		return err
	}

	if err = ivote.Close(ctx.GetSession(), state); err != nil {
		return
	}

	err = ctx.FollowUpEmbed(&discordgo.MessageEmbed{
		Description: "Vote closed.",
	}).Send().Error
	return
}
