package recorder

import "time"

// We don't expose these config options as flags.
type RecorderConfig struct {
	CreateStatement        string `flag:""`
	SummaryStatement       string `flag:""`
	InsertLatencyStatement string `flag:""`
	MinQuery               string `flag:""`
	MaxQuery               string `flag:""`
	WindowQuery            string `flag:""`
	RollupQuery            string `flag:""`
	MinRollupQuery         string `flag:""`
	RollupInsertStatement  string `flag:""`
	DeleteQuery            string `flag:""`
	OldRowsQuery           string `flag:""`

	WindowSize time.Duration `flag:"w" default:"2s"`

	DBPath string `flag:"db-path"`

	ChartPath string `flag:"chart" default:"" usage:"path to write chart to"`
}

func (r RecorderConfig) getDBPath() string {
	if r.DBPath != "" {
		return r.DBPath
	}

	return ":memory:"
}

func (c RecorderConfig) GetRecorderConfig() RecorderConfig {
	return c
}
