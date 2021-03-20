package main

import (
	"flag"
	"os"

	"kubevirt.io/client-go/log"
	virtlauncher "kubevirt.io/kubevirt/pkg/virt-launcher"
)

func main() {
	containerDiskDir := flag.String("container-disk-dir", "/var/run/kubevirt/container-disks", "Base directory for container disk data")
	flag.Parse()

	exitCode, err := virtlauncher.ForkAndMonitor(*containerDiskDir, false)
	if err != nil {
		log.Log.Reason(err).Error("monitoring virt-launcher failed")
		os.Exit(1)
	}
	os.Exit(exitCode)
	// cmd := exec.Command("/usr/bin/virt-launcher", append(os.Args[1:], "--no-fork", "true")...)
	// cmd.SysProcAttr = &syscall.SysProcAttr{
	// 	AmbientCaps: []uintptr{10},
	// }
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	// if err := cmd.Start(); err != nil {
	// 	log.Log.Reason(err).Error("failed to start virt-launcher")
	// 	os.Exit(1)
	// }
	// if err := cmd.Wait(); err != nil {
	// 	log.Log.Reason(err).Error("virt-launcher failed")
	// 	os.Exit(1)
	// }
	// os.Exit(0)

}
