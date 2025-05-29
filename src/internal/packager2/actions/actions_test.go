package actions

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func Test_actionCmdMutation(t *testing.T) {
	zarfCmd, err := utils.GetFinalExecutableCommand()
	require.NoError(t, err)
	tests := []struct {
		name      string
		cmd       string
		shellPref v1alpha1.Shell
		goos      string
		want      string
		wantErr   error
	}{
		{
			name:      "linux without zarf",
			cmd:       "echo \"this is zarf\"",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      "echo \"this is zarf\"",
			wantErr:   nil,
		},
		{
			name:      "linux including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   nil,
		},
		{
			name:      "windows including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   nil,
		},
		{
			name:      "windows env",
			cmd:       "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1",
			wantErr:   nil,
		},
		{
			name: "windows env pwsh",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "pwsh",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: nil,
		},
		{
			name: "windows env powershell",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "powershell",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: nil,
		},
		{
			name:      "windows multiple env",
			cmd:       "echo ${ZARF_VAR_ENV1} ${ZARF_VAR_ENV2}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1 $Env:ZARF_VAR_ENV2",
			wantErr:   nil,
		},
		{
			name:      "windows constants",
			cmd:       "echo ${ZARF_CONST_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_CONST_ENV1",
			wantErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actionCmdMutation(context.Background(), tt.cmd, tt.shellPref, tt.goos)
			require.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.want, got)
		})
	}
}
