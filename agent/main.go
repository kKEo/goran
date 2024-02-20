package main

import (
	"fmt"
	"time"

	"github.com/kkEo/g-mk8s/agent/config"
	"github.com/kkEo/g-mk8s/agent/events"
	"github.com/kkEo/g-mk8s/agent/process"
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

		p := process.Process{
			Name: "date",
		}
		p.Run()

		time.Sleep(2 * time.Second)
	}
}
