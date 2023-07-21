package listeners

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sarulabs/di/v2"
	"github.com/zekrotja/dgrs"
	"github.com/zekrotja/rogu"
	"github.com/zekrotja/rogu/log"

	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/services/guildlog"
	"github.com/zekroTJA/shinpuru/internal/services/timeprovider"
	"github.com/zekroTJA/shinpuru/internal/util/static"
)

type ListenerVoiceUpdate struct {
	db database.Database
	gl guildlog.Logger
	st *dgrs.State
	tp timeprovider.Provider

	cache    map[string]*discordgo.VoiceState
	cacheMtx sync.RWMutex

	log rogu.Logger
}

func NewListenerVoiceUpdate(container di.Container) *ListenerVoiceUpdate {
	return &ListenerVoiceUpdate{
		db: container.Get(static.DiDatabase).(database.Database),
		gl: container.Get(static.DiGuildLog).(guildlog.Logger).Section("voicelog"),
		st: container.Get(static.DiState).(*dgrs.State),
		tp: container.Get(static.DiTimeProvider).(timeprovider.Provider),

		cache: make(map[string]*discordgo.VoiceState),

		log: log.Tagged("VoiceLog"),
	}
}

func (l *ListenerVoiceUpdate) sendVLCMessage(s *discordgo.Session, channelID, userID, content string, color int) {
	user, err := l.st.User(userID)
	if err != nil {
		return
	}
	s.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Color:       color,
		Description: content,
		Author: &discordgo.MessageEmbedAuthor{
			Name:    user.String(),
			IconURL: user.AvatarURL("16x16"),
		},
		Timestamp: l.tp.Now().Format(time.RFC3339),
	})
}

func (l *ListenerVoiceUpdate) sendJoinMsg(s *discordgo.Session, voiceLogChan, userID string, newChan *discordgo.Channel) {
	msgTxt := fmt.Sprintf(":arrow_right:  Joined **`%s`**", newChan.Name)
	l.sendVLCMessage(s, voiceLogChan, userID, msgTxt, static.ColorEmbedGreen)
}

func (l *ListenerVoiceUpdate) sendMoveMsg(s *discordgo.Session, voiceLogChan, userID string, oldChan, newChan *discordgo.Channel) {
	msgTxt := fmt.Sprintf(":left_right_arrow:  Moved from **`%s`** to **`%s`**", oldChan.Name, newChan.Name)
	l.sendVLCMessage(s, voiceLogChan, userID, msgTxt, static.ColorEmbedCyan)
}

func (l *ListenerVoiceUpdate) sendLeaveMsg(s *discordgo.Session, voiceLogChan, userID string, oldChan *discordgo.Channel) {
	msgTxt := fmt.Sprintf(":arrow_left:  Left **`%s`**", oldChan.Name)
	l.sendVLCMessage(s, voiceLogChan, userID, msgTxt, static.ColorEmbedOrange)
}

func (l *ListenerVoiceUpdate) isBlocked(guildID, chanID string) (ok bool) {
	ok, err := l.db.IsGuildVoiceLogIgnored(guildID, chanID)
	if err != nil {
		log.Error().Tag("VoiceLog").Err(err).Msg("Failed getting blocked state")
		l.gl.Errorf(guildID, "Failed getting blocked state: %s", err.Error())
	}
	return
}

func (l *ListenerVoiceUpdate) Handler(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	l.cacheMtx.RLock()
	vsOld := l.cache[e.UserID]
	l.cacheMtx.RUnlock()

	vsNew := e.VoiceState
	if vsOld != nil && vsOld.ChannelID == vsNew.ChannelID {
		return
	}

	l.cacheMtx.Lock()
	l.cache[e.UserID] = vsNew
	l.cacheMtx.Unlock()

	voiceLogChan, err := l.db.GetGuildVoiceLog(e.GuildID)
	if err != nil || voiceLogChan == "" {
		return
	}

	_, err = l.st.Channel(voiceLogChan)
	if err != nil {
		l.db.SetGuildVoiceLog(e.GuildID, "")
		return
	}

	if vsOld == nil || vsOld.ChannelID == "" {
		l.log.Debug().Fields("vsOld", vsOld, "vsNew", vsNew).Msg("user joined VC")

		newChan, err := l.st.Channel(e.ChannelID)
		if err != nil {
			return
		}

		if l.isBlocked(newChan.GuildID, newChan.ID) {
			return
		}

		l.sendJoinMsg(s, voiceLogChan, e.UserID, newChan)

	} else if vsOld != nil && vsNew.ChannelID != "" && vsOld.ChannelID != vsNew.ChannelID {
		l.log.Debug().Fields("vsOld", vsOld, "vsNew", vsNew).Msg("user moved VC")

		newChan, err := l.st.Channel(vsNew.ChannelID)
		if err != nil {
			return
		}

		oldChan, err := l.st.Channel(vsOld.ChannelID)
		if err != nil {
			return
		}

		newChanBlocked := l.isBlocked(vsNew.GuildID, vsNew.ChannelID)
		oldChanBlocked := l.isBlocked(vsOld.GuildID, vsOld.ChannelID)

		if newChanBlocked && oldChanBlocked {
			// send no message
		} else if newChanBlocked {
			l.sendLeaveMsg(s, voiceLogChan, e.UserID, oldChan)
		} else if oldChanBlocked {
			l.sendJoinMsg(s, voiceLogChan, e.UserID, newChan)
		} else {
			l.sendMoveMsg(s, voiceLogChan, e.UserID, oldChan, newChan)
		}

	} else if vsOld != nil && vsNew.ChannelID == "" {
		l.log.Debug().Fields("vsOld", vsOld, "vsNew", vsNew).Msg("user left VC")

		oldChan, err := l.st.Channel(vsOld.ChannelID)
		if err != nil {
			return
		}

		if l.isBlocked(oldChan.GuildID, oldChan.ID) {
			return
		}

		l.sendLeaveMsg(s, voiceLogChan, e.UserID, oldChan)
	}
}
