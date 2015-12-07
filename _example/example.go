package main

import "github.com/mark-adams/announce-bot"

func main() {
	config, err := announcebot.LoadConfigFromEnv("EXAMPLE")
	if err != nil {
		panic(err)
	}

	bot := announcebot.NewAnnounceBot(*config)
	bot.MessageFactory = func() string {
		return "Hello! It worked!"
	}

	err = bot.ListenAndStart()

	if err != nil {
		panic(err)
	}
}
