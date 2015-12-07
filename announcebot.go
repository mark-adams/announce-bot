package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/mattn/go-xmpp"

	log "github.com/Sirupsen/logrus"
	health "github.com/docker/go-healthcheck"
	"gopkg.in/redis.v3"
)

// AnnounceBot represents a chat bot for making announcements
type AnnounceBot struct {
	Config             Configuration
	AnnounceHandler    http.HandlerFunc
	TestHandler        http.HandlerFunc
	HealthcheckHandler http.HandlerFunc

	subscriptionListHandler http.HandlerFunc
	subscriptionHandler     http.HandlerFunc

	listener net.Listener
	started  bool
}

// NewAnnounceBot instantiates a new AnnounceBot
func NewAnnounceBot(config Configuration) *AnnounceBot {
	return &AnnounceBot{
		Config:          config,
		AnnounceHandler: announce,
		TestHandler:     test,
	}
}

func announce(w http.ResponseWriter, r *http.Request) {

}

func test(w http.ResponseWriter, r *http.Request) {

}

func (ab *AnnounceBot) subscriptionList(w http.ResponseWriter, r *http.Request) {
	// client := redis.NewClient(ab.Config.GetRedisOptions())
}

func (ab *AnnounceBot) subscription(w http.ResponseWriter, r *http.Request) {

}

func (ab *AnnounceBot) withBotContext(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		context.Set(r, "db", redis.NewClient(ab.Config.GetRedisOptions()))
		h.ServeHTTP(w, r)
		context.Clear(r)
	}
}

func (ab *AnnounceBot) getRoutes() http.Handler {
	r := mux.NewRouter()

	r.HandleFunc("/subscribers", ab.subscriptionListHandler).Methods("GET")
	r.HandleFunc("/subscriber/{user_id}", ab.subscriptionHandler).Methods("GET", "DELETE", "PUT")
	r.HandleFunc("/announce", ab.withBotContext(ab.AnnounceHandler)).Methods("POST")
	r.HandleFunc("/test", ab.withBotContext(ab.TestHandler)).Methods("POST")
	r.HandleFunc("/debug/health", health.StatusHandler)

	return r
}

func (ab *AnnounceBot) handleChatMessage(talk *xmpp.Client, user string, message string) {
	userID := parseUser(user)
	log.WithField("user", userID).WithField("message", message).Info("Received message")
	talk.Send(xmpp.Chat{Remote: user, Type: "chat", Text: message})
}

func (ab *AnnounceBot) serveXMPP() {
	var talk *xmpp.Client
	var err error

	options := xmpp.Options{
		Host:     ab.Config.HipchatXMPPHost,
		User:     ab.Config.HipchatUser,
		Password: ab.Config.HipchatPassword,
		NoTLS:    true,
	}

	talk, err = options.NewClient()

	if err != nil {
		log.Fatal(err)
	}

	log.Info("XMPP client connected successfully")

	go func() {
		for {
			chat, err := talk.Recv()
			if err != nil {
				log.Fatal(err)
			}
			switch v := chat.(type) {
			case xmpp.Chat:
				ab.handleChatMessage(talk, v.Remote, v.Text)
			}
		}
	}()
}

// Start begins serving API requests and processing chat messages
func (ab *AnnounceBot) Start(l net.Listener) error {
	if ab.started {
		return errors.New("This bot has already been started")
	}
	ab.started = true

	// Start the chat server
	log.Info("Starting XMPP client")
	ab.serveXMPP()
	log.Info("XMPP client started succesfully")

	// Start the API server
	log.Info("Starting the API server")
	ab.listener = l
	r := ab.getRoutes()
	err := http.Serve(ab.listener, r)
	if err != nil {
		return err
	}

	return nil
}

// ListenAndStart starts the bot and begins listening for API requests
func (ab *AnnounceBot) ListenAndStart() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", ab.Config.Port))
	if err != nil {
		return err
	}

	return ab.Start(listener)
}
