package main

import "fmt"

func main() {

	fmt.Println("Running!")
	config, err := LoadConfigFromEnv("ANN")
	if err != nil {
		panic(err)
	}

	bot := NewAnnounceBot(*config)
	err = bot.ListenAndStart()

	if err != nil {
		panic(err)
	}
}
