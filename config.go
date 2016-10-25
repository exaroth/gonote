package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strings"
)

const (
	defaultConfigFilename = ".gonote.json"
	defaultMarkdownOption = true
	defaultEditorOption   = "vim"
)

// Main configuration interface used to interact with configuration file.
type MainConfig interface {
	Load() error
	GetUserConfig() *UserConfigFile
	read() error
	create() error
}

type mainConfig struct {
	Path    string
	UserCfg *UserConfigFile
}

// Structure representing user configuration file.
type UserConfigFile struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Markdown bool   `json:"markdown"`
	Editor   string `json:"editor"`
}

// Return new configation instance.
func NewConfigFile() MainConfig {
	usr, _ := user.Current()
	return &mainConfig{
		Path: path.Join(usr.HomeDir, defaultConfigFilename),
		UserCfg: &UserConfigFile{
			Markdown: defaultMarkdownOption,
			Editor:   defaultEditorOption,
		},
	}
}

// Retrieve user configuration file.
func (c *mainConfig) GetUserConfig() *UserConfigFile {
	return c.UserCfg
}

// Load settings from configutation file.
func (c *mainConfig) Load() (err error) {
	new_file := false
	// Check if file exists
	if _, err = os.Stat(c.Path); err != nil {
		new_file = true
	}

	if new_file {
		if err = c.create(); err != nil {
			return
		}

	} else {
		if err = c.read(); err != nil {
			return
		}
	}
	return
}

// Read configuration file from disk.
func (c *mainConfig) read() (err error) {
	if c.Path == "" {
		return errors.New("Missing path to configuration file")
	}
	file, err := os.Open(c.Path)
	if err != nil {
		return
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c.UserCfg)
	return
}

// Create new configuration file if not found
// in user directory.
func (c *mainConfig) create() (err error) {
	fmt.Println("Creating new GoNote configuration file")
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter SimpleNote email:")
	// TODO: Refactor it
	c.UserCfg.Email, err = reader.ReadString('\n')
	c.UserCfg.Email = strings.TrimSpace(c.UserCfg.Email)
	fmt.Println("Enter SimpleNote password:")
	c.UserCfg.Password, err = reader.ReadString('\n')
	c.UserCfg.Password = strings.TrimSpace(c.UserCfg.Password)
	c.UserCfg.Editor = defaultEditorOption
	if err != nil {
		return
	}
	f, err := json.MarshalIndent(c.UserCfg, "", "\t")
	err = ioutil.WriteFile(c.Path, f, 0751)
	return
}
