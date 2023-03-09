package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveMinerProposalToCSV(t *testing.T) {
	file := t.TempDir() + "t01001.csv"

	data := map[string]string{
		"":  "a",
		"b": "c",
	}
	assert.NoError(t, SaveMinerProposalToCSV(file, data))

	res, err := LoadMinerProposalFromCSV(file)
	assert.NoError(t, err)
	assert.Equal(t, "a", res[""])
	assert.Equal(t, "c", res["b"])
}
