package server

import (
	"log"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the default browser at the given URL (best effort).
func OpenBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		log.Printf("could not open browser: %v", err)
	}
}
