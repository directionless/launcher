package gsettings

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func xTestParseValue(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		in       []string
		outValue string
		outType  string
		err      error
	}{
		{
			in:       []string{"true"},
			outType:  "boolean",
			outValue: "true",
		},
		{
			in:       []string{"false"},
			outType:  "boolean",
			outValue: "false",
		},
		{
			in:       []string{"'hello'"},
			outType:  "string",
			outValue: "hello",
		},
		{
			in:       []string{"123"},
			outType:  "numeric",
			outValue: "123",
		},
		{
			in:       []string{"1.23"},
			outType:  "numeric",
			outValue: "1.23",
		},
		{
			in:       []string{"uint32", "0"},
			outType:  "uint32",
			outValue: "0",
		},
		{
			in:  []string{"too", "many", "components"},
			err: errors.New(""),
		},
		{
			in:  []string{},
			err: errors.New(""),
		},
		{
			in:  []string{"\t\n\t"},
			err: errors.New(""),
		},
	}

	tableInstance := &tableInstance{logger: log.NewJSONLogger(os.Stderr)}

	for _, tt := range tests {
		valType, val, err := tableInstance.parseValue(tt.in)
		if tt.err != nil {
			require.Error(t, err, "with input %+v", tt.in)
		} else {
			require.NoError(t, err, "with input %+v", tt.in)
			require.Equal(t, tt.outType, valType, "with input %+v", tt.in)
			require.Equal(t, tt.outValue, val, "with input %+v", tt.in)
		}
	}

}

func TestParseGSettingsSource1(t *testing.T) {
	t.Parallel()

	tableInstance := &tableInstance{logger: log.NewJSONLogger(os.Stderr)}

	testData, err := ioutil.ReadFile("testdata/org.gnome.desktop.interface.txt")
	require.NoError(t, err, "read test data")

	results, err := tableInstance.parseGSettings("testUser", string(testData))
	require.NoError(t, err, "parse test data")

	_ = results

}
