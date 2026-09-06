//go:build windows

package lcu

import "os/exec"

// fromProcess reads LeagueClientUx.exe's command line via WMI.
func fromProcess() (Creds, error) {
	out, err := exec.Command("wmic", "process", "where", "name='LeagueClientUx.exe'", "get", "commandline").Output()
	if err != nil {
		// wmic is absent on newer Windows builds; fall back to PowerShell CIM.
		out, err = exec.Command("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_Process -Filter \"name='LeagueClientUx.exe'\" | Select-Object -ExpandProperty CommandLine").Output()
		if err != nil {
			return Creds{}, err
		}
	}
	return parseProcessList(string(out))
}
