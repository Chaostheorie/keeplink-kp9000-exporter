package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Listen       string
	ScrapeConfig string
	Timeout      time.Duration
}

type ScrapeConfig struct {
	Switches []SwitchConfig `json:"switches"`
}

type SwitchConfig struct {
	SwitchIP     string        `json:"switch_ip"`
	UsernameFile string        `json:"username_file"`
	PasswordFile string        `json:"password_file"`
	Timeout      time.Duration `json:"timeout"`
}

// load credentials from username/password file
func (sc *SwitchConfig) Load() (*Switch, error) {
	username, err := os.ReadFile(sc.UsernameFile)
	if err != nil {
		slog.Error("failed to load username file", "path", sc.UsernameFile, "err", err)

		return nil, err
	}

	password, err := os.ReadFile(sc.PasswordFile)
	if err != nil {
		slog.Error("failed to load password file", "path", sc.PasswordFile, "err", err)

		return nil, err
	}

	username = bytes.TrimSpace(username)
	password = bytes.TrimSpace(password)

	sum := md5.Sum(append(username, password...))
	cookie := "admin=" + hex.EncodeToString(sum[:])

	client := new(http.Client)
	client.Timeout = sc.Timeout

	return &Switch{
		client: client,
		cookie: cookie,
		addr:   sc.SwitchIP,
	}, nil
}
