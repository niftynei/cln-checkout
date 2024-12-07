package main

import (
	"fmt"
	"log"
	"github.com/base58btc/cln-checkout"
)

func main() {
	log.SetPrefix("cln-checkout|")
	log.SetFlags(log.Lshortfile | log.Ltime | log.LUTC)

	err := checkout.Init("")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("checkout init")
}