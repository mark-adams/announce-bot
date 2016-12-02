package announcebot

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/mattn/go-xmpp"

	log "github.com/Sirupsen/logrus"
	health "github.com/docker/go-healthcheck"
	"github.com/tbruyelle/hipchat-go/hipchat"
)

func (bot *AnnounceBot) withBotContext(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		context.Set(r, "db", bot.db)
		context.Set(r, "chat_api", bot.chatAPI)
		context.Set(r, "config", &bot.Config)
		h.ServeHTTP(w, r)
		context.Clear(r)
	}
}

func (bot *AnnounceBot) getRoutes() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/subscribers", bot.withBotContext(bot.subscriptionListHandler)).Methods("GET")
	r.HandleFunc("/subscriber/{user_id}", bot.withBotContext(bot.subscriptionHandler)).Methods("GET", "DELETE", "POST", "PUT")
	r.HandleFunc("/announce", bot.withBotContext(bot.announceHandler)).Methods("POST")
	r.HandleFunc("/test", bot.withBotContext(bot.testHandler)).Methods("POST")
	r.HandleFunc("/debug/health", health.StatusHandler)

	return r
}

func (bot *AnnounceBot) announceHandler(w http.ResponseWriter, r *http.Request) {
	config := getConfiguration(r)

	subscribers, err := bot.getSubscribers()
	if err != nil {
		log.WithError(err).Error("An error occurred while retrieving subscribers")
		w.WriteHeader(204)
		return
	}

	message, err := bot.MessageFactory()
	if err != nil {
		log.WithError(err).Error("No message generated for the announcement")
		return
	}

	if config.AnnounceRoom != "-1" {
		_, err := bot.chatAPI.Room.Notification(
			config.AnnounceRoom,
			&hipchat.NotificationRequest{
				Message:       message,
				Notify:        true,
				MessageFormat: "text",
			},
		)
		if err != nil {
			log.WithError(err).Error("An error occurred while sending the announcement to the room")
		} else {
			log.WithField("room", config.AnnounceRoom).WithField("message", message).Info("Sent announcement to the room")
		}

	}

	go func() {
		talk, err := bot.getXMPPClient()
		if err != nil {
			log.WithError(err).WithField("message", message).Error("Error opening XMPP connection to send the announcement")
			return
		}
		for _, userID := range subscribers {
			userlog := log.WithField("user", userID).WithField("message", message)

			user, _, err := bot.chatAPI.User.View(userID)
			if err != nil {
				userlog.WithError(err).Error("Could not find the user via the Hipchat API")
				continue
			}

			_, err = talk.Send(xmpp.Chat{Remote: user.XMPPJid, Type: "chat", Text: message})
			if err != nil {
				userlog.WithError(err).Error("An error occurred while sending the announcement to a user")
				continue
			}
			userlog.Info("Sent announcement to user")
		}
	}()

	w.WriteHeader(204)
}

func (bot *AnnounceBot) testHandler(w http.ResponseWriter, r *http.Request) {
	_, err := bot.chatAPI.Room.Notification(
		bot.Config.TestRoom,
		&hipchat.NotificationRequest{
			Message: "This is a test message!",
			Notify:  true,
		},
	)
	if err != nil {
		log.WithError(err).Error("An error occurred while sending the test message")
	}
	w.WriteHeader(204)
}
