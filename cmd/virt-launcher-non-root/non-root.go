package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// allow bind dhcp server
// const CAP_NET_BIND_SERVICE = 10
const CAP_NET_ADMIN = 12

func main() {
	fmt.Println(os.Args[1:])
	cmd := exec.Command("/usr/bin/virt-launcher", os.Args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		AmbientCaps: []uintptr{CAP_NET_ADMIN},
	}
	err := cmd.Start()
	if err != nil {
		panic(err)
	}

	err = cmd.Wait()
	if err != nil {
		panic(err)
	}
	fmt.Println("Ended")
}
