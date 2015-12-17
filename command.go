package announcebot

import (
	"errors"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// ChatCommand receives a chat message and returns a reply
type ChatCommand func(user string, message string, logger *log.Entry) (reply string)

var errUserAlreadySubscribed = errors.New("User is already subscribed")
var errUserNotSubscribed = errors.New("User is not subscribed")

func (bot *AnnounceBot) subscribeCommand(user string, message string, logger *log.Entry) (reply string) {
	result, err := bot.subscribeUser(user)
	switch result {
	case 201:
		reply = "Alright, you're signed up!"
	case 400:
		reply = "Looks like you are already subscribed!"
	case 500:
		reply = "Uh oh... something didn't go right. Try again later. :-("
		if err != nil {
			logger.WithError(err).Error("Error while attempting to subscribe the user")
		}
	}

	return
}

func (bot *AnnounceBot) unsubscribeCommand(user string, message string, logger *log.Entry) (reply string) {
	result, err := bot.unsubscribeUser(user)
	switch result {
	case 204:
		reply = "Alright, you've been unsubscribed! I'l miss you..."
	case 400:
		reply = "Looks like you aren't subscribed!"
	case 500:
		reply = "Uh oh... something didn't go right. Try again later. :-("
		if err != nil {
			logger.WithError(err).Error("Error while attempting to unsubscribe the user")
		}
	}

	return
}

func (bot *AnnounceBot) helpCommand(user string, message string, logger *log.Entry) (reply string) {
	var validCommands []string
	for key := range bot.commands {
		validCommands = append(validCommands, key)
	}

	reply = fmt.Sprintf("Valid commands are: %s", strings.Join(validCommands, ", "))
	return
}
