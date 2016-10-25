package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

var Version = "0.1.0"

// List current GoNote version.
func ListVersion() string {
	return fmt.Sprintf("GoNote Ver.%s", Version)
}

// Generates unique filename for temp file.
func GenerateRandomHandle() string {
	return fmt.Sprintf("go_note_%s.txt", time.Now().Format("20060102150405"))
}

// Converts timestamp to Date readable to humans
func HumanDate(d string) string {
	t := time.Unix(GetSimpleNoteTimestamp(d), 0)
	return t.Format("2006-01-02 15:04:05")
}

// GetSimpleNoteTimestamp returns proper int timestamp parsed from SimpleNote date field.
func GetSimpleNoteTimestamp(d string) int64 {
	d = strings.Split(d, ".")[0] // Get only seconds
	i, _ := strconv.Atoi(d)
	return int64(i)
}

// Check if value is in array.
func CheckIn(needle string, haystack []string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// ToString converts some types used as flags values to string.
// We don't have to care about floats here (for now at least).
func ConvertToString(val interface{}) string {
	switch val.(type) {
	case string:
		return val.(string)
	case int:
		return strconv.Itoa(val.(int))
	case bool:
		return strconv.FormatBool(val.(bool))
	default:
		// Should never happen, will fail (most probably)
		return val.(string)
	}

}
func ParseTags(tags []string) (tagString string) {
	tc := make([]string, len(tags))
	for i, t := range tags {
		tc[i] = tagPrefix + t
	}
	return strings.Join(tc, ", ")
}

// writeToFile opens external editor with predetermined temporary file
// after editor is closed reads data from the file and deletes it.
func WriteToFile(prevContent, editor string) (content string, err error) {
	fpath := path.Join(os.TempDir(), GenerateRandomHandle())
	f, err := os.Create(fpath)
	if err != nil {
		return
	}
	defer f.Close()
	// This is used in edit action
	if prevContent != "" {
		_, err = f.WriteString(prevContent)
		if err != nil {
			return
		}
	}
	cmd := exec.Command(editor, fpath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return
	}
	err = cmd.Wait()
	if err != nil {
		return
	}
	c, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}
	return string(c), nil
}
