//go:build !windows

package lcu

import "os/exec"

// fromProcess scans the process table for LeagueClientUx arguments.
func fromProcess() (Creds, error) {
	out, err := exec.Command("ps", "-ax", "-o", "command").Output()
	if err != nil {
		return Creds{}, err
	}
	return parseProcessList(string(out))
}
