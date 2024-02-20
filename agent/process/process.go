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

	log.Printf("Running process: %s", p.Name)
	p.process = exec.Command(p.Name, p.Args...)

	stdoutPipe, err := p.process.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderrPipe, err := p.process.StderrPipe()
	if err != nil {
		panic(err)
	}

	if err := p.process.Start(); err != nil {
		panic(err)
	}

	var wait_group sync.WaitGroup
	wait_group.Add(2)

	p.stdoutChannel = make(chan string)
	go func() {
		log.Printf("Processing stdout")
		defer wait_group.Done()
		reader := bufio.NewReader(stdoutPipe)
		for {
			line, err := reader.ReadString('\n')
			log.Printf("Processing line: %s", line)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Fatal: %s", err)
			}
			p.stdoutChannel <- line
		}
		log.Printf("Process done - closing stdout")
		close(p.stdoutChannel)
	}()

	p.stderrChannel = make(chan string)
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
		close(p.stderrChannel)
	}()

	for line := range p.stdoutChannel {
		log.Printf("Stdout: %s", line)
	}

	for line := range p.stderrChannel {
		log.Printf("Stderr: %s", line)
	}

	log.Printf("Waiting for process to finish")
	p.process.Wait()

}
