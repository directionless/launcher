package gsettings

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/kolide/launcher/pkg/osquery/tables/tablehelpers"
	"github.com/kolide/osquery-go/plugin/table"
	"github.com/pkg/errors"
)

var numericRexexp = regexp.MustCompile(`^\d+(\.\d+)?$`) // regexp can't be constant

func GSettings(logger log.Logger) *table.Plugin {
	columns := []table.ColumnDefinition{
		table.TextColumn("username"),
		table.TextColumn("schema"),
		table.TextColumn("key"),
		table.TextColumn("value"),
		table.TextColumn("value_type"),
	}
	t := &tableInstance{logger: logger}
	return table.NewPlugin("kolide_gsettings", columns, t.generate)
}

type tableInstance struct {
	logger log.Logger
}

func (t *tableInstance) generate(ctx context.Context, queryContext table.QueryContext) ([]map[string]string, error) {
	var results []map[string]string

	usernames, err := tablehelpers.RequireConstraintEquals(queryContext, "username")
	if err != nil {
		return results, errors.Wrap(err, "kolide_gsettings")
	}

	schemas, err := tablehelpers.RequireConstraintEquals(queryContext, "schema")
	if err != nil {
		return results, errors.Wrap(err, "kolide_gsettings")
	}

	for _, username := range usernames {
		for _, schema := range schemas {
			// FIXME: This isn't going to handle usernames correctly. Needs sudo
			out, err := exec.CommandContext(ctx,
				"gsettings",
				"list-recursively",
				schema,
			).Output()
			if err != nil {
				return results, errors.Wrap(err, "exec gsettings")
			}

			newResults, err := t.parseGSettings(username, string(out))
			if err != nil {
				return results, errors.Wrap(err, "parse gsettings output")
			}

			results = append(results, newResults...)

		}
	}

	return results, nil
}

func (t *tableInstance) parseGSettings(username string, data string) ([]map[string]string, error) {
	var results []map[string]string

	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		components := strings.Fields(line)

		if len(components) < 3 {
			return results, errors.Errorf("Can't gsettings line: <%s>. Not enough components", line)
		}

		schema := components[0]
		key := components[1]
		valType, val, err := t.parseValue(components[2:])
		if err != nil {
			return results, errors.Wrapf(err, "parsing gsettings value from %+v", components[2:])
		}

		results = append(results, map[string]string{
			"username":   username,
			"schema":     schema,
			"key":        key,
			"value":      valType,
			"value_type": val,
		})
	}

	return results, nil
}

func (t *tableInstance) parseValue(components []string) (string, string, error) {
	switch {
	case len(components) == 0:
		return "", "", errors.New("Can't parse gsetting value from empty string")
	case len(components) > 2:
		return "", "", errors.Errorf("Can't parse gsetting value from %+v", components)
	case len(components) == 2:
		// two components means we have type data
		return components[0], components[1], nil
	}

	// Having covered the remaioning cases, there must only be 1
	// components. Attempt to determine what the type is.
	component := components[0]
	switch {
	case component == "true" || component == "false":
		return "boolean", component, nil
	case strings.HasPrefix(component, `'`) || strings.HasPrefix(component, `"`):
		return "string", strings.Trim(component, `'"`), nil
	case numericRexexp.MatchString(component):
		return "numeric", component, nil
	}

	level.Debug(t.logger).Log(
		"msg", "unable to parse type from gsettings",
		"components", fmt.Sprintf("%+v", component),
	)

	return "", "", errors.Errorf("Can't parse gsetting value from %+v", components)

}
