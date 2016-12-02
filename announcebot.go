package announcebot

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mattn/go-xmpp"

	log "github.com/Sirupsen/logrus"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"gopkg.in/redis.v3"
)

// ErrBotAlreadyStarted is returned when the bot has already been started
var ErrBotAlreadyStarted = errors.New("This bot has already been started")

// AnnounceBot represents a chat bot for making announcements
type AnnounceBot struct {
	Config         Configuration
	MessageFactory func() (string, error)

	commands map[string]ChatCommand
	listener net.Listener
	started  bool
	chatAPI  *hipchat.Client
	db       *redis.Client
}

// NewAnnounceBot instantiates a new AnnounceBot
func NewAnnounceBot(config Configuration) *AnnounceBot {
	bot := AnnounceBot{
		Config:         config,
		MessageFactory: defaultMessageFactory,
	}

	bot.commands = make(map[string]ChatCommand)
	bot.RegisterCommand("help", bot.helpCommand)
	bot.RegisterCommand("subscribe", bot.subscribeCommand)
	bot.RegisterCommand("unsubscribe", bot.unsubscribeCommand)

	return &bot
}

// RegisterCommand binds a ChatCommand to a specific keyword
func (bot *AnnounceBot) RegisterCommand(key string, cmd ChatCommand) {
	bot.commands[key] = cmd
}

func defaultMessageFactory() (string, error) {
	return "Something important happened!", nil
}

func (bot *AnnounceBot) handleChatMessage(talk *xmpp.Client, user string, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	userID := parseUser(user)

	userlog := log.WithField("user", userID)
	userlog.WithField("message", message).Info("Received message")

	if !bot.lockMessage(user, message) {
		userlog.Debug("Could not acquire lock... skipping")
		return
	}

	reply := fmt.Sprintf("I'm sorry. I didn't understand what you said. Try 'help' if you need to see a list of commands.")

	tokens := strings.Split(message, " ")
	keyword := strings.ToLower(tokens[0])
	handler, ok := bot.commands[keyword]
	if ok {
		reply = handler(userID, message, userlog)
	}

	if reply == "" {
		return
	}

	defer func() {
		userlog.WithField("message_reply", reply).Info("Replied to message")
	}()

	talk.Send(xmpp.Chat{Remote: user, Type: "chat", Text: reply})
}

func (bot *AnnounceBot) lockMessage(user string, message string) bool {
	result := bot.db.SetNX(fmt.Sprintf("lock::%s::%s", user, message), 1, 4*time.Second)
	if err := result.Err(); err != nil {
		log.WithError(err).Error("Could not aquire lock because of an error")
	}

	return result.Val()
}

// Start begins serving API requests and processing chat messages
func (bot *AnnounceBot) Start(l net.Listener) error {
	if bot.started {
		return ErrBotAlreadyStarted
	}

	bot.started = true
	bot.chatAPI = hipchat.NewClient(bot.Config.HipchatAPIToken)
	bot.db = redis.NewClient(bot.Config.GetRedisOptions())

	// Start the chat server
	if bot.Config.HipchatUser != "" {
		log.Info("Starting XMPP client")
		bot.serveXMPP()
		log.Info("XMPP client started succesfully")
	}

	// Start the API server
	log.Info("Starting the API server")
	bot.listener = l
	r := bot.getRoutes()

	return http.Serve(bot.listener, r)
}

// ListenAndStart starts the bot and begins listening for API requests
func (bot *AnnounceBot) ListenAndStart() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", bot.Config.Port))
	if err != nil {
		return err
	}

	return bot.Start(listener)
}
