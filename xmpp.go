package announcebot

import (
	"time"

	"github.com/mattn/go-xmpp"

	log "github.com/Sirupsen/logrus"
)

func (bot *AnnounceBot) getXMPPClient() (*xmpp.Client, error) {
	options := xmpp.Options{
		Host:     bot.Config.HipchatXMPPHost,
		User:     bot.Config.HipchatUser,
		Password: bot.Config.HipchatPassword,
		NoTLS:    true,
	}

	return options.NewClient()
}

func (bot *AnnounceBot) serveXMPP() {
	talk, err := bot.getXMPPClient()
	if err != nil {
		log.Fatal(err)
	}

	log.Info("XMPP client connected successfully")

	go func() {
		for {
			talk.PingC2S(bot.Config.HipchatUser, bot.Config.HipchatXMPPHost)
			log.Debug("Sent XMPP ping")
			time.Sleep(30 * time.Second)
		}
	}()

	go func() {
		for {
			chat, err := talk.Recv()
			if err != nil {
				log.Fatal(err)
			}
			log.WithField("chat_message", chat).Debug("Received XMPP message")

			switch v := chat.(type) {
			case xmpp.Chat:
				bot.handleChatMessage(talk, v.Remote, v.Text)
			}
		}
	}()
}
