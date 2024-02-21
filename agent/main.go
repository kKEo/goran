package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kkEo/g-mk8s/agent/client"
	"github.com/kkEo/g-mk8s/agent/config"
	"github.com/kkEo/g-mk8s/agent/events"
)

func main() {
	conf := config.AppConfig()

	emitter := events.Emitter{}

	var counter int

	for true {
		counter += 1
		event := events.Event{
			time.Now(),
			fmt.Sprintf("[%s]Round: %d", conf.ApiKey, counter),
		}
		emitter.Emit(&event)

		// p := process.Process{
		// 	Name: "date",
		// }
		// p.Run()

		c := client.Client{
			Url:    "http://localhost:8080/protected/next",
			ApiKey: "letmein_IknowTheSecret",
		}

		next := c.Next()
		if next != nil {
			log.Printf("Run: %v", next)
		}

		time.Sleep(2 * time.Second)
	}
}
