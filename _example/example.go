package main

import "github.com/mark-adams/announce-bot"

func main() {
	config, err := announcebot.LoadConfigFromEnv("EXAMPLE")
	if err != nil {
		panic(err)
	}

	bot := announcebot.NewAnnounceBot(*config)

	// Set your own message factory returning a string for
	//
	bot.MessageFactory = func() (string, error) {
		return "Hello! It worked!", nil
	}

	err = bot.ListenAndStart()

	if err != nil {
		panic(err)
	}
}
