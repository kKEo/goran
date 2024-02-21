package process

import (
	"bufio"
	"io"
	"log"
	"os/exec"
	"sync"
)

type Process struct {
	Name string
	Args []string

	process       *exec.Cmd
	stdoutChannel chan string
	stderrChannel chan string
}

func (p *Process) Run() {

	var wait_group sync.WaitGroup
	wait_group.Add(2)

	log.Printf("Running process: %s", p.Name)
	p.process = exec.Command(p.Name, p.Args...)

	p.stdoutChannel = make(chan string)

	stdoutPipe, err := p.process.StdoutPipe()
	if err != nil {
		panic(err)
	}

	go func() {
		log.Printf("Processing stdout")
		defer wait_group.Done()
		reader := bufio.NewReader(stdoutPipe)
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Fatal: %s", err)
			}
			log.Printf("Processing line: %s", line)
			p.stdoutChannel <- line
		}
		log.Printf("Process done - closing stdout")
		close(p.stdoutChannel)
	}()

	p.stderrChannel = make(chan string)
	stderrPipe, err := p.process.StderrPipe()
	if err != nil {
		panic(err)
	}

	go func() {
		defer wait_group.Done()
		reader := bufio.NewReader(stderrPipe)
		for {
			line, err := reader.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				panic(err)
			}
			p.stderrChannel <- line
		}
		log.Printf("Process done - closing stderr")
		close(p.stderrChannel)
	}()

	if err := p.process.Start(); err != nil {
		panic(err)
	}

	for line := range p.stdoutChannel {
		log.Printf(" -> Consumed: %s", line)
	}

	for line := range p.stderrChannel {
		log.Printf(" -> Consumed: %s", line)
	}

	log.Printf("Waiting for process to finish")
	p.process.Wait()
	wait_group.Wait()
}
