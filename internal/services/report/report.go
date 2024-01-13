package report

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/bwmarrin/snowflake"
	"github.com/sarulabs/di/v2"
	"github.com/zekroTJA/shinpuru/internal/models"
	"github.com/zekroTJA/shinpuru/internal/services/config"
	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/services/timeprovider"
	"github.com/zekroTJA/shinpuru/internal/util/snowflakenodes"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekroTJA/shinpuru/pkg/discordutil"
	"github.com/zekroTJA/shinpuru/pkg/inline"
	"github.com/zekroTJA/shinpuru/pkg/multierror"
	"github.com/zekroTJA/shinpuru/pkg/roleutil"
	"github.com/zekroTJA/shinpuru/pkg/stringutil"
	"github.com/zekrotja/dgrs"
	"github.com/zekrotja/rogu"
	"github.com/zekrotja/rogu/log"
)

var (
	ErrRoleDiff       = errors.New("you can only ban or kick members with lower permissions than yours")
	ErrMemberHasLeft  = errors.New("this user is no more a member of this guild")
	ErrInvalidTimeout = errors.New("timeout must be in the future")
)

type ReportService struct {
	s   discordutil.ISession
	db  database.Database
	cfg config.Provider
	st  dgrs.IState
	tp  timeprovider.Provider
	log rogu.Logger
}

type ReportError struct {
	error
	models.Report
}

func New(container di.Container) (t *ReportService, err error) {
	snowflakenodes.NodesReport = make([]*snowflake.Node, len(models.ReportTypes))
	for i, t := range models.ReportTypes {
		if snowflakenodes.NodesReport[i], err = snowflakenodes.RegisterNode(i, "report."+strings.ToLower(t)); err != nil {
			return nil, err
		}
	}

	return &ReportService{
		s:   container.Get(static.DiDiscordSession).(discordutil.ISession),
		db:  container.Get(static.DiDatabase).(database.Database),
		cfg: container.Get(static.DiConfig).(config.Provider),
		st:  container.Get(static.DiState).(dgrs.IState),
		tp:  container.Get(static.DiTimeProvider).(timeprovider.Provider),
		log: log.Tagged("Reports"),
	}, nil
}

// PushReport creates a new Report object with the given executorID,
// victimID, reason, attachmentID, and typ. The report is saved to the database
// using the passed db databse rpovider and an embed is created with the attachment
// url assembled with publicAddr as image endpoint root. This embed is then sent to
// the specified mod log channel for this guild, if existent.
func (r *ReportService) PushReport(rep models.Report) (models.Report, error) {
	repID := snowflakenodes.NodesReport[rep.Type].Generate()

	rep.ID = repID

	err := r.db.AddReport(rep)
	if err != nil {
		return models.Report{}, err
	}

	var modlogChan string
	if modlogChan, err = r.db.GetGuildModLog(rep.GuildID); err == nil && modlogChan != "" {
		_, err = r.s.ChannelMessageSendEmbed(modlogChan, rep.AsEmbed(r.cfg.Config().WebServer.PublicAddr))
	}
	if err != nil {
		if database.IsErrDatabaseNotFound(err) {
			err = nil
		} else {
			err = fmt.Errorf("failed sending message to modlog channel: %s", err)
		}
	}

	dmChan, errDm := r.s.UserChannelCreate(rep.VictimID)
	if errDm == nil && dmChan != nil {
		r.s.ChannelMessageSendEmbed(dmChan.ID, rep.AsEmbed(r.cfg.Config().WebServer.PublicAddr))
	}

	return rep, errors.Join(err, errDm)
}

// PushKick is shorthand for PushReport as member kick action and also
// kicks the member from the guild with the given reason and case ID
// for the audit log.
func (r *ReportService) PushKick(rep models.Report) (models.Report, error) {
	const typ = 0
	rep.Type = typ

	guild, err := r.st.Guild(rep.GuildID, true)
	if err != nil {
		return models.Report{}, err
	}

	victim, err := r.st.Member(rep.GuildID, rep.VictimID)
	if discordutil.IsErrCode(err, discordgo.ErrCodeUnknownMember) {
		return models.Report{}, ErrMemberHasLeft
	}
	if err != nil {
		return models.Report{}, err
	}

	executor, err := r.st.Member(rep.GuildID, rep.ExecutorID)
	if err != nil {
		return models.Report{}, err
	}

	isAdmin := discordutil.IsAdmin(guild, executor)
	if !isAdmin && roleutil.PositionDiff(victim, executor, guild) >= 0 {
		return models.Report{}, ErrRoleDiff
	}

	rep, err = r.PushReport(rep)
	if err != nil {
		return models.Report{}, err
	}

	if err = r.s.GuildMemberDeleteWithReason(rep.GuildID, rep.VictimID, fmt.Sprintf(`[CASE %s] %s`, rep.ID, rep.Msg)); err != nil {
		r.db.DeleteReport(rep.ID)
		return models.Report{}, err
	}

	return rep, nil
}

// PushBan is shorthand for PushReport as member ban action and also
// bans the member from the guild with the given reason and case ID
// for the audit log.
func (r *ReportService) PushBan(rep models.Report) (models.Report, error) {
	const typ = 1
	rep.Type = typ

	if err := checkTimeout(r.tp.Now(), rep.Timeout); err != nil {
		return models.Report{}, err
	}

	guild, err := r.st.Guild(rep.GuildID, true)
	if err != nil {
		return models.Report{}, err
	}

	var victim *discordgo.Member
	if !rep.Anonymous {
		victim, err = r.st.Member(rep.GuildID, rep.VictimID)
		if discordutil.IsErrCode(err, discordgo.ErrCodeUnknownMember) {
			rep.Anonymous = true
			err = nil
		}
		if err != nil {
			return models.Report{}, err
		}
	}

	executor, err := r.st.Member(rep.GuildID, rep.ExecutorID)
	if err != nil {
		return models.Report{}, err
	}

	if !rep.Anonymous {
		isAdmin := discordutil.IsAdmin(guild, executor)
		diff := roleutil.PositionDiff(victim, executor, guild)
		if !isAdmin && diff >= 0 {
			return models.Report{}, ErrRoleDiff
		}
	}

	rep, err = r.PushReport(rep)
	if err != nil {
		return models.Report{}, err
	}

	if err = r.s.GuildBanCreateWithReason(rep.GuildID, rep.VictimID, fmt.Sprintf(`[CASE %s] %s`, rep.ID, rep.Msg), 7); err != nil {
		r.db.DeleteReport(rep.ID)
		return models.Report{}, err
	}

	return rep, nil
}

// PushMute is shorthand for PushReport as member mute action and also
// adds the mute role to the specified victim.
func (r *ReportService) PushMute(rep models.Report) (models.Report, error) {
	const typ = 2
	rep.Type = typ

	if err := checkTimeout(r.tp.Now(), rep.Timeout); err != nil {
		return models.Report{}, err
	}

	guild, err := r.st.Guild(rep.GuildID, true)
	if err != nil {
		return models.Report{}, err
	}

	victim, err := r.st.Member(rep.GuildID, rep.VictimID)
	if discordutil.IsErrCode(err, discordgo.ErrCodeUnknownMember) {
		return models.Report{}, ErrMemberHasLeft
	}
	if err != nil {
		return models.Report{}, err
	}

	executor, err := r.st.Member(rep.GuildID, rep.ExecutorID)
	if err != nil {
		return models.Report{}, err
	}

	isAdmin := discordutil.IsAdmin(guild, executor)
	diff := roleutil.PositionDiff(victim, executor, guild)
	if !isAdmin && diff >= 0 {
		return models.Report{}, ErrRoleDiff
	}

	if rep.Msg == "" {
		rep.Msg = "no reason specified"
	}

	rep, err = r.PushReport(rep)
	if err != nil {
		return models.Report{}, err
	}

	err = r.s.GuildMemberTimeout(rep.GuildID, rep.VictimID, rep.Timeout)
	if err != nil {
		r.db.DeleteReport(rep.ID)
		return models.Report{}, err
	}

	return rep, nil
}

// RevokeMute removes the mute role of the specified victim and sends
// an unmute embed to the users DMs and to the mod log channel.
func (r *ReportService) RevokeMute(guildID, executorID, victimID, reason string) (emb *discordgo.MessageEmbed, err error) {
	guild, err := r.st.Guild(guildID, true)
	if err != nil {
		return nil, err
	}

	victim, err := r.st.Member(guildID, victimID)
	if err != nil {
		return nil, err
	}

	executor, err := r.st.Member(guildID, executorID)
	if err != nil {
		return nil, err
	}

	isAdmin := discordutil.IsAdmin(guild, executor)
	diff := roleutil.PositionDiff(victim, executor, guild)
	if !isAdmin && diff >= 0 {
		return nil, ErrRoleDiff
	}

	err = r.s.GuildMemberTimeout(guildID, victimID, nil)
	if err != nil {
		return
	}

	if err = r.ExpireLastReport(guildID, victimID, models.TypeMute); err != nil {
		return
	}

	repType := stringutil.IndexOf("MUTE", models.ReportTypes)
	repID := snowflakenodes.NodesReport[repType].Generate()

	if reason == "" {
		reason = "MANUAL UNMUTE"
	}

	emb = &discordgo.MessageEmbed{
		Title: "Case " + repID.String(),
		Color: models.ReportColors[repType],
		Fields: []*discordgo.MessageEmbedField{
			{
				Inline: true,
				Name:   "Executor",
				Value:  fmt.Sprintf("<@%s>", executorID),
			},
			{
				Inline: true,
				Name:   "Target",
				Value:  fmt.Sprintf("<@%s>", victimID),
			},
			{
				Name:  "Type",
				Value: "UNMUTE",
			},
			{
				Name:  "Description",
				Value: reason,
			},
		},
		Timestamp: time.Unix(repID.Time()/1000, 0).Format(time.RFC3339),
	}

	var modlogChan string
	if modlogChan, err = r.db.GetGuildModLog(guildID); err == nil {
		_, err = r.s.ChannelMessageSendEmbed(modlogChan, emb)
	}
	if err != nil {
		if database.IsErrDatabaseNotFound(err) {
			err = nil
		} else {
			err = fmt.Errorf("failed sending message to modlog channel: %s", err)
		}
	}

	dmChan, errDm := r.s.UserChannelCreate(victimID)
	if errDm == nil {
		r.s.ChannelMessageSendEmbed(dmChan.ID, emb)
	}

	return emb, errors.Join(err, errDm)
}

func (r *ReportService) RevokeReport(
	rep models.Report,
	executorID string,
	reason string,
	wsPublicAddr string,
) (emb *discordgo.MessageEmbed, err error) {

	if err = r.db.DeleteReport(rep.ID); err != nil {
		return
	}

	if rep.Type == models.TypeBan {
		if err = r.s.GuildBanDelete(rep.GuildID, rep.VictimID); err != nil {
			return
		}
	}

	emb = &discordgo.MessageEmbed{
		Color:       static.ReportRevokedColor,
		Title:       "REPORT REVOCATION",
		Description: "Revoked reports are deleted from the database and no more visible in any commands.",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Revoke Executor",
				Value: fmt.Sprintf("<@%s>", executorID),
			},
			{
				Name:  "Revocation Reason",
				Value: reason,
			},
			rep.AsEmbedField(wsPublicAddr),
		},
	}

	var modlogChan string
	if modlogChan, err = r.db.GetGuildModLog(rep.GuildID); err == nil {
		_, err = r.s.ChannelMessageSendEmbed(modlogChan, emb)
	}
	if err != nil {
		if database.IsErrDatabaseNotFound(err) {
			err = nil
		} else {
			err = fmt.Errorf("failed sending message to modlog channel: %s", err)
		}
	}

	dmChan, errDm := r.s.UserChannelCreate(rep.VictimID)
	if errDm == nil {
		r.s.ChannelMessageSendEmbed(dmChan.ID, emb)
	}

	return emb, errors.Join(err, errDm)
}

func (r *ReportService) UnbanReport(
	unbanReq models.UnbanRequest,
	executorID string,
	reason string,
	isUnban bool,
) (emb *discordgo.MessageEmbed, err error) {

	newRep := models.Report{
		Type:       inline.II(isUnban, models.TypeUnban, models.TypeUnbanRejected),
		GuildID:    unbanReq.GuildID,
		ExecutorID: executorID,
		VictimID:   unbanReq.UserID,
		Msg:        reason,
	}

	_, err = r.PushReport(newRep)
	if err != nil {
		return nil, err
	}

	return
}

func (r *ReportService) ExpireLastReport(guildID, victimID string, typ models.ReportType) (err error) {
	reps, err := r.db.GetReportsFiltered(guildID, victimID, typ, 0, 1)
	if err != nil {
		return
	}
	if len(reps) > 0 && reps[0].Timeout != nil {
		err = r.db.ExpireReports(reps[0].ID.String())
	}
	return
}

func (r *ReportService) ExpireExpiredReports() (mErr *multierror.MultiError) {
	mErr = multierror.New()

	reps, err := r.db.GetExpiredReports()
	mErr.Append(err)
	r.log.Debug().Field("n", len(reps)).Msg("Start expiring cleanup ...")

	expIDs := make([]string, 0, len(reps))
	for _, rep := range reps {
		r.log.Debug().Fields(
			"id", rep.ID,
			"typ", rep.Type,
		).Msg("Expiring report")
		err = r.revokeReportOnExpiration(rep)
		if err != nil && !discordutil.IsErrCode(err, discordgo.ErrCodeUnknownBan) {
			mErr.Append(&ReportError{
				error:  err,
				Report: rep,
			})
			continue
		}
		expIDs = append(expIDs, rep.ID.String())
	}

	mErr.Append(
		r.db.ExpireReports(expIDs...))

	return
}

func (r *ReportService) revokeReportOnExpiration(rep models.Report) (err error) {
	switch rep.Type {
	case models.TypeBan:
		err = r.s.GuildBanDelete(rep.GuildID, rep.VictimID)
	case models.TypeMute:
		_, err = r.RevokeMute(rep.GuildID, rep.ExecutorID, rep.VictimID, "Automatic Timeout")
	}
	return
}

func checkTimeout(now time.Time, t *time.Time) (err error) {
	if t != nil && t.Before(now) {
		err = ErrInvalidTimeout
	}
	return
}
