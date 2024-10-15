package logger

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_New(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Empty level, format, and destination are ok",
			cfg:  Config{},
		},
		{
			name: "Default config is ok",
			cfg:  ConfigDefault(),
		},
		{
			name: "Debug logs are ok",
			cfg: Config{
				Level: Debug,
			},
		},
		{
			name: "Info logs are ok",
			cfg: Config{
				Level: Info,
			},
		},
		{
			name: "Warn logs are ok",
			cfg: Config{
				Level: Warn,
			},
		},
		{
			name: "Error logs are ok",
			cfg: Config{
				Level: Error,
			},
		},
		{
			name: "Text format is supported",
			cfg: Config{
				Format: FormatText,
			},
		},
		{
			name: "JSON format is supported",
			cfg: Config{
				Format: FormatJSON,
			},
		},
		{
			name: "FormatNone is supported to disable logs",
			cfg: Config{
				Format: FormatNone,
			},
		},
		{
			name: "DestinationNone is supported to disable logs",
			cfg: Config{
				Destination: DestinationNone,
			},
		},
		{
			name: "users can send logs to any io.Writer",
			cfg: Config{
				Destination: os.Stdout,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := New(tc.cfg)
			assert.NoError(t, err)
			assert.NotNil(t, res)
		})
	}
}

func Test_NewErrors(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name string
		cfg  Config
	}{
		{
			name: "unsupported log level errors",
			cfg: Config{
				Level: 3,
			},
		},
		{
			name: "wildly unsupported log level errors",
			cfg: Config{
				Level: 42389412389213489,
			},
		},
		{
			name: "unsupported format errors",
			cfg: Config{
				Format: "foobar",
			},
		},
		{
			name: "wildly unsupported format errors",
			cfg: Config{
				Format: "^\\w+([-+.']\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$ lorem ipsum dolor sit amet 243897 )*&($#",
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := New(tc.cfg)
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}
}
