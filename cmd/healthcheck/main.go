package main

import (
	"docker-hytale-server/internal/utils"
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/charmbracelet/log"
)

func main() {
	logPrefix := "[HEALTHCHECK]"

	cmd := exec.Command("pgrep", "-f", "java.*HytaleServer")
	if err := cmd.Run(); err != nil {
		log.WithPrefix(logPrefix).Info("UNHEALTHY: Java process not running")
		os.Exit(1)
	}

	port := utils.GetEnv("SERVER_PORT", "5520")

	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%s", port))
	if err == nil {
		conn.Close()
		os.Exit(1)
	}

	log.WithPrefix(logPrefix).Info("HEALTHY")
	os.Exit(0)
}
