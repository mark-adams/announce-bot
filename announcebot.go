package announcebot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/mattn/go-xmpp"

	log "github.com/Sirupsen/logrus"
	health "github.com/docker/go-healthcheck"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"gopkg.in/redis.v3"
)

// AnnounceBot represents a chat bot for making announcements
type AnnounceBot struct {
	Config         Configuration
	MessageFactory func() (string, error)

	listener net.Listener
	started  bool
	chatAPI  *hipchat.Client
	db       *redis.Client
}

// NewAnnounceBot instantiates a new AnnounceBot
func NewAnnounceBot(config Configuration) *AnnounceBot {
	return &AnnounceBot{
		Config:         config,
		MessageFactory: defaultMessageFactory,
	}
}

func defaultMessageFactory() (string, error) {
	return "Something important happened!", nil
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
		_, err := bot.chatAPI.Room.Notification(config.AnnounceRoom, &hipchat.NotificationRequest{Message: message})
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

			_, err = talk.Send(xmpp.Chat{Remote: user.XmppJid, Type: "chat", Text: message})
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
	_, err := bot.chatAPI.Room.Notification(bot.Config.TestRoom, &hipchat.NotificationRequest{Message: "This is a test message!"})
	if err != nil {
		log.WithError(err).Error("An errorr occurred while sending the test message")
	}
	w.WriteHeader(204)
}

func (bot *AnnounceBot) subscriptionListHandler(w http.ResponseWriter, r *http.Request) {
	subscribers, err := bot.getSubscribers()
	if err != nil {
		log.WithError(err).Error("Error retrieving subscribers")
		w.WriteHeader(500)
	}

	data, err := json.Marshal(subscribers)
	if err != nil {
		log.WithError(err).Error("Error during JSON serialization")
		w.WriteHeader(500)
		return
	}
	w.Write(data)
}

func (bot *AnnounceBot) getSubscribers() ([]string, error) {
	result := bot.db.SMembers("subscribers")
	err := result.Err()
	if err != nil {
		return nil, err
	}

	return result.Val(), err
}

func (bot *AnnounceBot) subscribeUser(userID string) (int, error) {
	existsCmd := bot.db.SIsMember("subscribers", userID)
	err := existsCmd.Err()
	if err != nil {
		return 500, err
	}

	if existsCmd.Val() {
		return 400, errors.New("User is already subscribed")
	}

	addCmd := bot.db.SAdd("subscribers", userID)
	err = addCmd.Err()
	if err != nil {
		return 500, err
	}

	return 201, nil
}

func (bot *AnnounceBot) unsubscribeUser(userID string) (int, error) {
	existsCmd := bot.db.SIsMember("subscribers", userID)
	err := existsCmd.Err()
	if err != nil {
		return 500, err
	}

	if !existsCmd.Val() {
		return 400, errors.New("User is not subscribed")
	}

	remCmd := bot.db.SRem("subscribers", userID)
	err = remCmd.Err()
	if err != nil {
		return 500, err
	}

	return 204, nil
}

func (bot *AnnounceBot) subscriptionHandler(w http.ResponseWriter, r *http.Request) {
	var retCode int
	var err error

	vars := mux.Vars(r)
	userID := vars["user_id"]

	if r.Method == "PUT" {
		retCode, err = bot.subscribeUser(userID)
	} else if r.Method == "DELETE" {
		retCode, err = bot.unsubscribeUser(userID)
	}

	if err != nil {
		w.WriteHeader(retCode)
		if retCode != 500 {
			w.Write([]byte(err.Error()))
		}
		return
	}

	w.WriteHeader(retCode)
}

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
	r.HandleFunc("/subscriber/{user_id}", bot.withBotContext(bot.subscriptionHandler)).Methods("GET", "DELETE", "PUT")
	r.HandleFunc("/announce", bot.withBotContext(bot.announceHandler)).Methods("POST")
	r.HandleFunc("/test", bot.withBotContext(bot.testHandler)).Methods("POST")
	r.HandleFunc("/debug/health", health.StatusHandler)

	return r
}

func (bot *AnnounceBot) handleChatMessage(talk *xmpp.Client, user string, message string) {
	reply := "I'm sorry. I didn't understand what you said. Try 'subscribe' or 'unsubscribe'"

	if message == "" {
		return
	}

	userID := parseUser(user)

	userlog := log.WithField("user", userID)
	userlog.WithField("message", message).Info("Received message")
	defer func() {
		userlog.WithField("message_reply", reply).Info("Replied to message")
	}()

	switch message {
	case "subscribe":
		result, err := bot.subscribeUser(userID)
		switch result {
		case 201:
			reply = "Alright, you're signed up!"
		case 400:
			reply = "Looks like you are already subscribed!"
		case 500:
			reply = "Uh oh... something didn't go right. Try again later. :-("
			if err != nil {
				userlog.WithError(err).Error("Error while attempting to subscribe the user")
			}
		}
	case "unsubscribe":
		result, err := bot.unsubscribeUser(userID)
		switch result {
		case 204:
			reply = "Alright, you've been unsubscribed! I'l miss you..."
		case 400:
			reply = "Looks like you aren't subscribed!"
		case 500:
			reply = "Uh oh... something didn't go right. Try again later. :-("
			if err != nil {
				userlog.WithError(err).Error("Error while attempting to unsubscribe the user")
			}
		}
	}
	talk.Send(xmpp.Chat{Remote: user, Type: "chat", Text: reply})
}

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
			switch v := chat.(type) {
			case xmpp.Chat:
				bot.handleChatMessage(talk, v.Remote, v.Text)
			}
		}
	}()
}

// Start begins serving API requests and processing chat messages
func (bot *AnnounceBot) Start(l net.Listener) error {
	if bot.started {
		return errors.New("This bot has already been started")
	}

	bot.started = true
	bot.chatAPI = hipchat.NewClient(bot.Config.HipchatAPIToken)
	bot.db = redis.NewClient(bot.Config.GetRedisOptions())

	// Start the chat server
	log.Info("Starting XMPP client")
	bot.serveXMPP()
	log.Info("XMPP client started succesfully")

	// Start the API server
	log.Info("Starting the API server")
	bot.listener = l
	r := bot.getRoutes()
	err := http.Serve(bot.listener, r)
	if err != nil {
		return err
	}

	return nil
}

// ListenAndStart starts the bot and begins listening for API requests
func (bot *AnnounceBot) ListenAndStart() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", bot.Config.Port))
	if err != nil {
		return err
	}

	return bot.Start(listener)
}
