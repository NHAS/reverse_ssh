package client

import (
	"strings"
	"testing"
)

func TestParseNTLMCreds(t *testing.T) {
	tests := []struct {
		name          string
		creds         string
		wantDomain    string
		wantUser      string
		wantPass      string
		wantErr       bool
		expectedError string
	}{
		{
			name:          "Valid credentials",
			creds:         "DOMAIN\\user:pass",
			wantDomain:    "DOMAIN",
			wantUser:      "user",
			wantPass:      "pass",
			wantErr:       false,
			expectedError: "",
		},
		{
			name:          "Empty credentials",
			creds:         "",
			wantErr:       true,
			expectedError: "NTLM credentials not provided",
		},
		{
			name:          "Missing domain",
			creds:         "user:pass",
			wantErr:       true,
			expectedError: "invalid NTLM credentials format",
		},
		{
			name:          "Missing password",
			creds:         "DOMAIN\\user",
			wantErr:       true,
			expectedError: "invalid NTLM credentials format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, user, pass, err := parseNTLMCreds(tt.creds)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if domain != tt.wantDomain {
				t.Errorf("Domain = %q, want %q", domain, tt.wantDomain)
			}
			if user != tt.wantUser {
				t.Errorf("User = %q, want %q", user, tt.wantUser)
			}
			if pass != tt.wantPass {
				t.Errorf("Pass = %q, want %q", pass, tt.wantPass)
			}
		})
	}
}
