package say

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"log/slog"
	"os"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "say",
		Short: "He's just a little guy", // FIXME(mkcp)
		Long:  "He's really just a little guy!",
		RunE: func(cmd *cobra.Command, args []string) error {
			zarfLogo := message.GetLogo()
			_, err := fmt.Fprintln(os.Stderr, zarfLogo)
			l := cmd.Context().Value("logger").(*slog.Logger)
			l.Debug("Done printing that scamperoni", "guyStatus", "little")
			return err
		},
	}
}
