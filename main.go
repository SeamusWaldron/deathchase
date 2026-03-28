package main

import (
	"log"

	"deathchase/game"
)

func main() {
	if err := game.Run(); err != nil {
		log.Fatal(err)
	}
}
