package ephemeraldiskutils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type idOwnershipManager struct {
	uid, gid int
}

func (manager *idOwnershipManager) SetFileOwnership(file string) error {
	return os.Chown(file, manager.uid, manager.gid)
}

// func OwnerShipManagerForVMI(vmi *v1.VirtualMachineInstance) OwnershipManagerInterface {
// 	return &idOwnershipManager{uid: uid, gid: gid}
// }

func OwnerShipManagerFor(uid, gid int) OwnershipManagerInterface {
	return &idOwnershipManager{uid: uid, gid: gid}
}

const effectiveId = 1

// GetUidAndGidFor uses /proc/<pid>/status to obtain effective uid and gid
func GetUidAndGidFor(pid int) (int, int, error) {
	statusFile := filepath.Join("/proc", fmt.Sprint(pid), "status")
	file, err := os.Open(statusFile)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	// format of status file is ->Uid:	107	107	107	107
	uid, gid := -1, -1
	scan := bufio.NewScanner(file)
	for scan.Scan() {
		line := scan.Text()
		if strings.Contains(line, "Uid:") {
			uids := strings.Split(line, " ")[1:]
			val, err := strconv.Atoi(uids[effectiveId])
			if err != nil {
				return 0, 0, err
			}
			uid = val
			continue
		}
		if strings.Contains(line, "Gid:") {
			gids := strings.Split(line, " ")[1:]
			val, err := strconv.Atoi(gids[effectiveId])
			if err != nil {
				return 0, 0, err
			}
			gid = val
			continue
		}
	}
	if uid == -1 || gid == -1 {
		return uid, gid, fmt.Errorf("Failed to find uid or gid")
	}
	return uid, gid, nil

}
