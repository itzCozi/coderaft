package commands

import (
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid alphanumeric name",
			projectName: "myproject",
			wantErr:     false,
		},
		{
			name:        "valid name with numbers",
			projectName: "project123",
			wantErr:     false,
		},
		{
			name:        "valid name with hyphens",
			projectName: "my-project",
			wantErr:     false,
		},
		{
			name:        "valid name with underscores",
			projectName: "my_project",
			wantErr:     false,
		},
		{
			name:        "valid mixed name",
			projectName: "my-project_123",
			wantErr:     false,
		},
		{
			name:        "empty name",
			projectName: "",
			wantErr:     true,
			errContains: "project name cannot be empty",
		},
		{
			name:        "name with spaces",
			projectName: "my project",
			wantErr:     true,
			errContains: "project name can only contain alphanumeric characters, hyphens, and underscores",
		},
		{
			name:        "name with special characters",
			projectName: "my@project",
			wantErr:     true,
			errContains: "project name can only contain alphanumeric characters, hyphens, and underscores",
		},
		{
			name:        "name with dots",
			projectName: "my.project",
			wantErr:     true,
			errContains: "project name can only contain alphanumeric characters, hyphens, and underscores",
		},
		{
			name:        "name with forward slash",
			projectName: "my/project",
			wantErr:     true,
			errContains: "project name can only contain alphanumeric characters, hyphens, and underscores",
		},
		{
			name:        "name with backslash",
			projectName: "my\\project",
			wantErr:     true,
			errContains: "project name can only contain alphanumeric characters, hyphens, and underscores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectName(tt.projectName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateProjectName() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("validateProjectName() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateProjectName() unexpected error = %v", err)
				}
			}
		})
	}
}
