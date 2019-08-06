package tablehelpers

import (
	"github.com/kolide/osquery-go/plugin/table"
	"github.com/pkg/errors"
)

func RequireConstraintEquals(queryContext table.QueryContext, columnName string) ([]string, error) {
	var values []string

	q, ok := queryContext.Constraints["columnName"]
	if !ok || len(q.Constraints) == 0 {
		return []string{}, errors.Errorf("Missing constraint: %s is required", columnName)
	}

	for _, constraint := range q.Constraints {
		if constraint.Operator != table.OperatorEquals {
			return []string{}, errors.Errorf("Unsupported operator: %s only accepts = constraints", columnName)
		}

		values = append(values, constraint.Expression)
	}

	return values, nil
}
