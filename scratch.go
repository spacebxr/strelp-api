package main

import (
	"fmt"
	"github.com/spacebxr/strelp-api/internal/discord"
)

func main() {
	p, err := discord.FetchProfile("1300318105116872758")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("%+v\n", p)
}
