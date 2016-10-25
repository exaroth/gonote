package main

import (
	"log"
)

func main() {

	var err error
	config := NewConfigFile()
	err = config.Load()
	if err != nil {
		log.Fatal(err)
	}
	commandLineParser := newCommandLineParser(config)
	params, err := commandLineParser.Grab()
	if err != nil {
		log.Fatal(err)
	}
	if params.Content == "" && params.Action == "" {
		// This happens when user did not enter anything in editor - don't send empty note then.
		return
	}
	simpleNoteClient := newSimpleNoteClient(nil, config, params)
	simpleNoteClient.Authorize()
	err = simpleNoteClient.Handle()
	if err != nil {
		log.Fatal(err)
	}
}
