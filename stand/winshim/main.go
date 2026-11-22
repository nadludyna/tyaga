// yt-dlp.exe for the Windows test stand: runs stand/fake-yt-dlp with Python.
package main

import (
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("py", append([]string{"-I", os.Getenv("FAKE_YTDLP")}, os.Args[1:]...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}
