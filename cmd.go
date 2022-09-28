package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	tagPrefix           = "@" // Prefix for tags passed by user
	SimpleNoteKeyLength = 32  // Length of note keys in SimpleNote
	maxStdinLen         = 1024 * 1024
)

var (
	// Actions available for the user. Keys represent name of actions, values, whether action requires note key parameter.
	CustomActions = &map[string]bool{
		"version": false,
		"list":    false,
		"delete":  true,
		"edit":    true,
		"get":     true,
	}
)

// cmmandLineParser contains both user parameters and configuration file.
type commandLineParser struct {
	Params *CommandLineParams
	config *UserConfigFile
}

// Command line params represents all actions available to the user from command line.
type CommandLineParams struct {
	Content string            // Content of the note to be saved
	Tags    []string          // List of tags to be used with requests
	Action  string            // Action represents custom action performed by user
	Key     string            // For some actions Note key is required
	Flags   map[string]string // Flags are additional params passed with some commands
	Piped   bool
}

// Interface implementing methods used for command line interaction.
type CommandLineParser interface {
	Grab() (*CommandLineParams, error)
	getTags([]string) []string
	getAction([]string) ([]string, error)
	getFlags([]string) []string
}

// NewCommandLineParser returns new command parser interface.
func newCommandLineParser(config MainConfig) (c CommandLineParser) {
	return &commandLineParser{
		Params: &CommandLineParams{
			Flags: make(map[string]string),
		},
		config: config.GetUserConfig(),
	}
}

// Read retrieves all parameters passed from command line.
func (c *commandLineParser) Grab() (params *CommandLineParams, err error) {
	args := os.Args[1:]
	err = c.parse(args)
	return c.Params, err
}

//Get flags retrieves all flags passed by the user.
func (c *commandLineParser) getFlags(args []string) []string {
	var flagListItemCount int
	var flagListShowDeleted, flagDeletePermanently bool
	cmdFlagSet := flag.NewFlagSet("", flag.ExitOnError)
	cmdFlagSet.IntVar(&flagListItemCount, "n", -1, "Number of items to show with list command.")
	cmdFlagSet.BoolVar(&flagListShowDeleted, "deleted", false, "Whether to show deleted items with list command.")
	cmdFlagSet.BoolVar(&flagDeletePermanently, "permanently", false, "If true will permanently delete the note instead of moving it to trash.")
	cmdFlagSet.Parse(args)
	c.Params.Flags["n"] = ConvertToString(flagListItemCount)
	c.Params.Flags["deleted"] = ConvertToString(flagListShowDeleted)
	c.Params.Flags["permanently"] = ConvertToString(flagDeletePermanently)
	// Return all remaining arguments
	return cmdFlagSet.Args()
}

// Check if arguments have any tags defined if so pop it from the list and save.
// Tags are always the first parameter or after keyword.
func (c *commandLineParser) getTags(args []string) []string {
	if len(args) > 0 {
		for i, arg := range args {
			if strings.HasPrefix(arg, tagPrefix) {
				c.Params.Tags = append(c.Params.Tags, strings.TrimPrefix(arg, tagPrefix))
				// Meaning user passed only tags
				if i == len(args)-1 {
					return []string{}
				}
			} else {
				return args[i:]
			}
		}
	}
	return args
}

// GetAction retrieves custom user action from command line.
func (c *commandLineParser) getAction(args []string) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}
	for action, requiresKey := range *CustomActions {
		if action == args[0] {
			c.Params.Action = action
			if requiresKey {
				if len(args) < 2 {
					return nil, errors.New("Missing note key parameter.")
				}
				if len(args[1]) != SimpleNoteKeyLength {
					return nil, errors.New("Invalid identifier passed")
				}
				c.Params.Key = strings.TrimSpace(args[1])
				return args[2:], nil
			}
			return args[1:], nil
		}
	}
	return args, nil
}

// GetStdin retrieves STDIN string if available
func (c *commandLineParser) getStdin() (in string, err error) {
	if c.Params.Piped {
		reader := bufio.NewReader(os.Stdin)
		buf := make([]byte, maxStdinLen)
		for {
			n, err := reader.Read(buf[:cap(buf)])
			if err != nil {
				fmt.Println(err.Error())
			}
			buf = buf[:n]
			if n == 0 {
				if err != nil {
					continue
				}
				if err == io.EOF {
					break
				}
				return "", err
			}
			if err != nil && err != io.EOF {
				return "", err
			}
			return string(buf), nil
		}
	}
	return "", nil
}

// Parse retrieves content of a note to be saved.
func (c *commandLineParser) parse(args []string) (err error) {
	stat, _ := os.Stdin.Stat()
	c.Params.Piped = ((stat.Mode() & os.ModeCharDevice) == 0)
	actionless, err := c.getAction(args)
	if err != nil {
		return err
	}
	tagless := c.getTags(actionless)
	flagless := c.getFlags(tagless)
	// If action is defined we don't need any content passed
	if c.Params.Piped {
		content, err := c.getStdin()
		if err != nil {
			return err
		}
		c.Params.Content = content
	} else if c.Params.Action == "" {
		if len(flagless) > 0 {
			c.Params.Content = strings.Join(flagless, " ")
		} else {
			content, err := WriteToFile("")
			if err != nil {
				return err
			}
			c.Params.Content = strings.TrimSpace(content)
		}
	} else {
		return
	}
	return
}
