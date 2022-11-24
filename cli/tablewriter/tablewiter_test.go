package tablewriter

import (
	"os"
	"testing"

	"github.com/fatih/color"
)

func TestTableWriter(t *testing.T) {
	tw := New(Col("Head1"), Col("Head won't show up "), Col("Head3"), NewLineCol("New Line Head"))
	tw.Write(map[string]interface{}{
		"Head1": "any",
		"Head3": "any",
	})
	tw.Write(map[string]interface{}{
		"Head1":         "any",
		"Head3":         "any",
		"Surprise Head": color.GreenString("#"),
		"New Line Head": "short value",
	})
	tw.Write(map[string]interface{}{
		"Head1":         "any",
		"Head3":         "a very very very very long value",
		"New Line Head": "a very very very very long value",
	})
	if err := tw.Flush(os.Stdout); err != nil {
		t.Fatal(err)
	}
}
