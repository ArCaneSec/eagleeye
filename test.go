package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	sig := make(chan os.Signal, 1)

	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
	}

	signal.Notify(sig, signals...)

	cmd := exec.Command("bash", "-c", "sleep 5; echo 'done'")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	var out bytes.Buffer
	cmd.Stdout = &out

	go func(sig chan os.Signal, cmd *exec.Cmd) {
		<-sig
		signal.Stop(sig)

		// err := cmd.Process.Signal(os.Kill)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		log.Println("signal received")
	}(sig, cmd)

	err := cmd.Start()
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	cmd.Wait()
	fmt.Println(out.String())
}
