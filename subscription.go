package announcebot

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

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
		return 400, errUserAlreadySubscribed
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
		return 400, errUserNotSubscribed
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

	switch r.Method {
	case "POST":
	case "PUT":
		retCode, err = bot.subscribeUser(userID)
	case "DELETE":
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
