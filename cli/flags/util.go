package flags

import (
	"strings"

	"github.com/urfave/cli"
)

func eachName(longName string, fn func(string)) {
	parts := strings.Split(longName, ",")
	for _, name := range parts {
		name = strings.Trim(name, " ")
		fn(name)
	}
}

// MarkRequired marks flags with specified names as required.
func MarkRequired(flagSet []cli.Flag, names ...string) []cli.Flag {
	updatedflagSet := make([]cli.Flag, 0, len(flagSet))
	for _, flag := range flagSet {
		for _, n := range names {
			if n == flag.GetName() {
				switch f := (flag).(type) {
				case cli.StringFlag:
					f.Required = true
					flag = f
				case cli.IntFlag:
					f.Required = true
					flag = f
				case cli.BoolFlag:
					f.Required = true
					flag = f
				}
				break
			}
		}
		updatedflagSet = append(updatedflagSet, flag)
	}
	return updatedflagSet
}
