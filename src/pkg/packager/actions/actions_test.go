package actions

import (
	"context"
	"fmt"
	"testing"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

func Test_actionCmdMutation(t *testing.T) {
	zarfCmd, _ := utils.GetFinalExecutableCommand()
	tests := []struct {
		name      string
		cmd       string
		shellPref v1alpha1.Shell
		goos      string
		want      string
		wantErr   bool
	}{
		{
			name:      "linux without zarf",
			cmd:       "echo \"this is zarf\"",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      "echo \"this is zarf\"",
			wantErr:   false,
		},
		{
			name:      "linux including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "linux",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   false,
		},
		{
			name:      "windows including zarf",
			cmd:       "./zarf deploy",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      fmt.Sprintf("%s deploy", zarfCmd),
			wantErr:   false,
		},
		{
			name:      "windows env",
			cmd:       "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1",
			wantErr:   false,
		},
		{
			name: "windows env pwsh",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "pwsh",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: false,
		},
		{
			name: "windows env powershell",
			cmd:  "echo ${ZARF_VAR_ENV1}",
			shellPref: v1alpha1.Shell{
				Windows: "powershell",
			},
			goos:    "windows",
			want:    "echo $Env:ZARF_VAR_ENV1",
			wantErr: false,
		},
		{
			name:      "windows multiple env",
			cmd:       "echo ${ZARF_VAR_ENV1} ${ZARF_VAR_ENV2}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_VAR_ENV1 $Env:ZARF_VAR_ENV2",
			wantErr:   false,
		},
		{
			name:      "windows constants",
			cmd:       "echo ${ZARF_CONST_ENV1}",
			shellPref: v1alpha1.Shell{},
			goos:      "windows",
			want:      "echo $Env:ZARF_CONST_ENV1",
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := actionCmdMutation(context.Background(), tt.cmd, tt.shellPref, tt.goos)
			if (err != nil) != tt.wantErr {
				t.Errorf("actionCmdMutation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("actionCmdMutation() got = %v, want %v", got, tt.want)
			}
		})
	}
}
