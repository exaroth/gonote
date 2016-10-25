package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	baseUrl                 = "https://simple-note.appspot.com/api2/"
	authorizeUrl            = "https://simple-note.appspot.com/api/login"
	dataEndpoint            = "data"
	indexEndpoint           = "index"
	defaultNoteAmount       = 100
	defaultNoteFetchTimeout = 10 // Max time wait to retrieve all the notes, in secs.
	noteHeaderLength        = 80 // Max number of characters to be used in note header
)

var (
	// Default headers sent with each request to SimpleNote servers.
	simpleNoteHeaders = map[string]string{
		"User-Agent":   fmt.Sprintf("GoNote/%s", Version),
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}
	blueColored = color.New(color.FgBlue).SprintFunc()
	redColored  = color.New(color.FgRed).SprintFunc()
	cyanColored = color.New(color.FgCyan).SprintFunc()
)

// Note represents note object returned by SimpleNote API.
type Note struct {
	Content    string   `json:"content"`
	Tags       []string `json:"tags"`
	SystemTags []string `json:"systemtags"`
	Deleted    int      `json:"deleted,omitempty"`
	Key        string   `json:"key,omitempty"`
	ShareKey   string   `json:"sharekey,omitempty"`
	PublishKey string   `json:"publishkey,omitempty"`
	ModifyDate string   `json:"modifydate"`
	CreateDate string   `json:"createdate"`
}

type Notes []Note // Need it for sorting

// NoteList represents list object returned by SimpleNote when calling note list endpoint.
type NoteList struct {
	Count int    `json:"count"`
	Data  []Note `json:"data"`
	Mark  string `json:"mark"`
}

var (
	noteListBody = `Showing %s notes for %s:
==================================
%s
`
	noteListRecord = `%s
%s %s
%s
----------------------------------`
)

// Interface representing client used for connecting with simplenote.
type SimpleNoteClient interface {
	Authorize() error
	Handle() error
	makeRequest(string, string, io.Reader, map[string]string) ([]byte, error, int)
	parseAddr(*http.Request, map[string]string)
	getAllNotes(Notes, string) (Notes, error)
	fetchNote(*Note) Note
	listNotes() error
	createNote() (*Note, error)
	deleteNote() error
	updateNote(n *Note) error
	editNote() error
	showNotes(notes Notes)
	showNote(note *Note)
}

// simpleNoteClient represents struct containing all data needed for
// calling SimpleNote.
type simpleNoteClient struct {
	Client *http.Client
	Token  string
	Cfg    *UserConfigFile
	Params *CommandLineParams
}

// newSimpleNoteClient returns client used for communicating with SimpleNote.
func newSimpleNoteClient(httpClient *http.Client, config MainConfig, params *CommandLineParams) SimpleNoteClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &simpleNoteClient{
		Client: httpClient,
		Cfg:    config.GetUserConfig(),
		Params: params,
	}
}

// Basic Handler delegating actions passed by the user.
func (s *simpleNoteClient) Handle() error {
	// Check for list or other parameters and call action
	// If not create new note
	if s.Params.Action != "" {
		switch s.Params.Action {
		case "version":
			fmt.Println(ListVersion())
		case "list":
			return s.listNotes()
		case "edit":
			return s.editNote()
		case "delete":
			return s.deleteNote()
		case "get":
			n := &Note{
				Key: s.Params.Key,
			}
			retrieved := s.fetchNote(n)
			s.showNote(&retrieved)
			return nil
		}
	} else {
		newNote, err := s.createNote()
		if err != nil {
			return err
		}
		s.showNote(newNote)
		return nil
	}
	return nil
}

// UpdateNote updates all available values for given note.
func (s *simpleNoteClient) updateNote(n *Note) (err error) {
	if s.Params.Key == "" { // Should never happen
		return errors.New("Missing key parameter in request.")
	}
	data, err := json.Marshal(n)
	if err != nil {
		return
	}
	_, err, code := s.makeRequest(fmt.Sprintf("%s%s/%s", baseUrl, dataEndpoint, n.Key), http.MethodPost, bytes.NewReader(data), nil)
	if code != http.StatusOK {
		return errors.New(fmt.Sprintf("Error when updating note, code was: %d", code))
	}
	fmt.Println("Note updated.")
	return
}

// EditNote edits the note in given editor then updates it's contents in SimpleNote.
func (s *simpleNoteClient) editNote() (err error) {
	// TODO: update version for the note to allow reverting to previous version.
	n := &Note{
		Key: s.Params.Key,
	}
	note := s.fetchNote(n)
	updatedContent, err := WriteToFile(note.Content, s.Cfg.Editor)
	if err != nil {
		return err
	}
	note.Content = strings.TrimSpace(updatedContent)
	return s.updateNote(&note)
}

// DeleteNote deletes the note with given key
func (s *simpleNoteClient) deleteNote() (err error) {
	// TODO: update version for the note to allow reverting to previous version.
	n := &Note{
		Key: s.Params.Key,
	}
	note := s.fetchNote(n)
	note.Deleted = 1
	if err = s.updateNote(&note); err != nil {
		return
	}
	if s.Params.Flags["permanently"] == "true" {
		// Permanently delete the note
		_, err, code := s.makeRequest(fmt.Sprintf("%s%s/%s", baseUrl, dataEndpoint, note.Key), http.MethodDelete, nil, nil)
		if err != nil {
			return err
		}
		if code != http.StatusOK {
			return errors.New(fmt.Sprintf("Simplenote request failed. Code was: %d", code))
		}
	}
	return

}

// ListNotes fetches all user notes and displays them in terminal.
func (s *simpleNoteClient) listNotes() (err error) {
	notes := []Note{}
	notes, err = s.getAllNotes(notes, "")
	noteCh := make(chan Note, len(notes))
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		s.showNotes([]Note{})
		return
	}
	for _, n := range notes {
		go func(n Note) {
			noteCh <- s.fetchNote(&n)
		}(n)
	}
	reqTimeout := time.After(defaultNoteFetchTimeout * time.Second)
	fullNotes := []Note{}
	for {
		select {
		case retrieved := <-noteCh:
			fullNotes = append(fullNotes, retrieved)
			if len(fullNotes) == len(notes) {
				s.showNotes(fullNotes)
				return
			}
		case <-reqTimeout:
			return errors.New("Timeout when fetching notes")
		}
	}
}

// ParseNote returns prettified version of the note record.
func (s *simpleNoteClient) parseNote(note *Note, shorten bool) string {
	// Generally we should not have notes with whitespace at the end or beggining...
	// But it won't hurt :D...
	lines := strings.Split(note.Content, "\n")
	for i, line := range lines {
		if line != "" {
			lines = lines[i:]
			break
		}
	}
	var content string
	if len(lines) > 0 {
		if shorten {
			if noteHeaderLength < len(lines[0]) {
				content = lines[0][:noteHeaderLength] + "..."
			} else {
				content = lines[0]
			}
		} else {
			content = strings.Join(lines, "\n")
		}

	}
	return fmt.Sprintf(noteListRecord, redColored(note.Key), cyanColored(HumanDate(note.ModifyDate)), blueColored(ParseTags(note.Tags)), content)
}

// ShowNotes displays fetched list of notes to the user.
func (s *simpleNoteClient) showNotes(notes Notes) {
	sort.Sort(notes)
	parsed := []string{}
	for _, n := range notes {
		if n.Content != "" { // Don't show empty notes
			parsed = append(parsed, s.parseNote(&n, true))
		}
	}
	if len(parsed) == 0 {
		fmt.Printf(noteListBody, 0, s.Cfg.Email, "No Notes")
		return
	}
	if s.Params.Flags["n"] != "-1" {
		n, err := strconv.Atoi(s.Params.Flags["n"])
		if err != nil {
			n = len(parsed)
		}
		if n > len(parsed) {
			n = len(parsed)
		}
		cut := len(parsed) - n

		parsed = parsed[cut:]
	}
	fmt.Printf(noteListBody, blueColored(len(parsed)), s.Cfg.Email, strings.Join(parsed, "\n"))
	return
}

// Show note prints single note to the user
func (s *simpleNoteClient) showNote(note *Note) {
	fmt.Printf(s.parseNote(note, false))
	return
}

// createNote creates new note using SimpleNote API.
func (s *simpleNoteClient) createNote() (n *Note, err error) {
	n = &Note{
		Content:    s.Params.Content,
		Tags:       s.Params.Tags,
		SystemTags: []string{},
	}
	if s.Cfg.Markdown {
		n.SystemTags = append(n.SystemTags, "markdown")
	}
	data, err := json.Marshal(n)
	if err != nil {
		return
	}
	resp, err, code := s.makeRequest(fmt.Sprintf("%s%s", baseUrl, dataEndpoint), http.MethodPost, bytes.NewReader(data), nil)
	if err != nil {
		return
	} else if code != http.StatusOK {
		return n, errors.New(fmt.Sprintf("Simplenote request failed. Code was: %d", code))
	}
	// When creating the note we don't get 'content' field in return so we have to copy it.
	// In order to print it back to the user.
	newNote := Note{}
	if err = json.Unmarshal(resp, &newNote); err != nil {
		return
	}
	newNote.Content = n.Content
	// We don't actually need any response data when creating new notes.
	// TODO: if we use shortened urls we'll need to update keys file at this time.
	return &newNote, nil
}

// FetchNote retrieves single note contents.
func (s *simpleNoteClient) fetchNote(n *Note) Note {
	resp, err, code := s.makeRequest(fmt.Sprintf("%s%s/%s", baseUrl, dataEndpoint, n.Key), http.MethodGet, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	i := Note{}
	if code != http.StatusOK {
		err = errors.New(fmt.Sprintf("Simplenote request failed. Code was: %d", code))
		log.Fatal(err)
	} else if err = json.Unmarshal(resp, &i); err != nil {
		log.Fatal(err)
	}
	return i
}

// GetAllNotes retrieves all notes from SimpleNote user account.
func (s *simpleNoteClient) getAllNotes(notes Notes, mark string) (Notes, error) {
	qparams := map[string]string{"length": strconv.Itoa(defaultNoteAmount)}
	if mark != "" {
		qparams["mark"] = mark
	}
	resp, err, code := s.makeRequest(fmt.Sprintf("%s%s", baseUrl, indexEndpoint), http.MethodGet, nil, qparams)
	if err != nil {
		return []Note{}, err

	} else if code != http.StatusOK {
		return []Note{}, errors.New(fmt.Sprintf("Simplenote request failed. Code was: %s", code))
	}
	l := NoteList{}
	if err = json.Unmarshal(resp, &l); err != nil {
		return []Note{}, err
	}
	for _, n := range l.Data {
		// Filter out notes by passed tags if any.
		if s.Params.Flags["deleted"] != "true" && n.Deleted == 1 {
			continue
		}
		if len(s.Params.Tags) > 0 {
			for _, t := range s.Params.Tags {
				if CheckIn(t, n.Tags) {
					notes = append(notes, n)
					break
				}
			}
		} else {
			notes = append(notes, n)
		}
	}
	if l.Mark != "" {
		// Retrieve next part of the list and append.
		newNotes, err := s.getAllNotes(notes, mark)
		if err != nil {
			return []Note{}, err
		}
		notes = append(notes, newNotes...)
	}
	return notes, nil
}

// Authorize retrieves access token used for calling SimpleNote servers.
func (s *simpleNoteClient) Authorize() (err error) {
	body := fmt.Sprintf("email=%s&password=%s", s.Cfg.Email, s.Cfg.Password)
	encodedBody := base64.StdEncoding.EncodeToString([]byte(body))
	req, err := http.NewRequest(http.MethodPost, authorizeUrl, strings.NewReader(encodedBody))
	if err != nil {
		return
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Error authorizing the client, check if username or password are valid. Status code was : %d", resp.StatusCode))
	}
	code, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	s.Token = string(code)
	return
}

func (s *simpleNoteClient) parseAddr(req *http.Request, params map[string]string) {
	vals := req.URL.Query()
	p := map[string]string{
		"auth":  s.Token,
		"email": s.Cfg.Email,
	}
	if params != nil {
		for k, v := range params {
			p[k] = v
		}
	}
	for k, v := range p {
		vals.Add(k, v)
	}
	req.URL.RawQuery = vals.Encode()
}

// Basic HTTP handler used for all SimpleNote requests (except Authorize).
func (s *simpleNoteClient) makeRequest(addr, method string, body io.Reader, additionalParams map[string]string) (response []byte, err error, code int) {
	req, err := http.NewRequest(method, addr, body)
	if err != nil {
		return
	}
	s.parseAddr(req, additionalParams)
	for headerName, headerVal := range simpleNoteHeaders {
		req.Header.Set(headerName, headerVal)
	}
	resp, err := s.Client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		if err = s.Authorize(); err != nil {
			return
		}
		return s.makeRequest(addr, method, body, additionalParams)
	} else if resp.StatusCode == http.StatusInternalServerError || resp.StatusCode == http.StatusPreconditionFailed {
		return s.makeRequest(addr, method, body, additionalParams)
	}
	response, err = ioutil.ReadAll(resp.Body)
	return response, err, resp.StatusCode
}

func (notes Notes) Len() int {
	return len(notes)
}

func (notes Notes) Less(i, j int) bool {
	return GetSimpleNoteTimestamp(notes[i].ModifyDate) < GetSimpleNoteTimestamp(notes[j].ModifyDate)
}

func (notes Notes) Swap(i, j int) {
	notes[i], notes[j] = notes[j], notes[i]
	return
}
