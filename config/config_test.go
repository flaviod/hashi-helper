package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_ParseContent(t *testing.T) {
	tests := []struct {
		name             string
		env              string
		content          string
		seenEnvironments []string
		seenApplications []string
		seenSecrets      []string
		wantErr          bool
	}{
		// wildcard and named environment mixed, should expose the seen environment
		// as the "test" since "*" matches that
		{
			name: "parse simple",
			env:  "test",
			content: `
environment "*" {
	application "seatgeek" {
		secret "very-secret" {
			value = "hello world"
		}
	}
}`,
			seenEnvironments: []string{"test"},
			seenApplications: []string{"seatgeek"},
			seenSecrets:      []string{"very-secret"},

			wantErr: false,
		},
		//
		{
			name: "parse multi with match",
			env:  "prod",
			content: `
environment "prod" "stag" {
	application "seatgeek" {
		secret "very-secret" {
			value = "hello world"
		}
	}
}`,
			seenEnvironments: []string{"prod"},
			seenApplications: []string{"seatgeek"},
			seenSecrets:      []string{"very-secret"},

			wantErr: false,
		},
		{
			name: "parse multi with _no_ match",
			env:  "perf",
			content: `
environment "prod" "stag" {
	application "seatgeek" {
		secret "very-secret" {
			value = "hello world"
		}
	}
}`,
			seenEnvironments: []string{},
			seenApplications: []string{},
			seenSecrets:      []string{},

			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				targetEnvironment: tt.env,
			}

			got, err := c.parseContent(tt.content, "test.hcl")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			err2 := c.processContent(got, "test.hcl")
			if tt.wantErr {
				require.Error(t, err2)
			} else {
				require.NoError(t, err2)
			}

			require.Equal(t, tt.seenEnvironments, c.Environments.list())
			require.Equal(t, tt.seenApplications, c.Applications.list())
			require.Equal(t, tt.seenSecrets, c.VaultSecrets.List())
		})
	}
}

func TestConfig_renderContent(t *testing.T) {
	tests := []struct {
		name              string
		template          string
		templateVariables map[string]interface{}
		wantTemplate      string
		wantErr           error
	}{
		{
			name:         "no templating, passthrough",
			template:     `hello="world"`,
			wantTemplate: `hello = "world"`,
		},
		{
			name:         "test service func missing consul_domain",
			template:     `service = "[[ service "derp" ]]"`,
			wantTemplate: `service = "derp.service.consul"`,
		},
		{
			name:     "test template func: service",
			template: `service="[[ service "vault" ]]"`,
			templateVariables: map[string]interface{}{
				"consul_domain": "test.consul",
			},
			wantTemplate: `service = "vault.service.test.consul"`,
		},
		{
			name:     "test template func: serviceWithTag",
			template: `service="[[ serviceWithTag "vault" "active" ]]"`,
			templateVariables: map[string]interface{}{
				"consul_domain": "test.consul",
			},
			wantTemplate: `service = "active.vault.service.test.consul"`,
		},
		{
			name:     "test template func: grantCredentials",
			template: `[[ grantCredentials "my-db" "full" ]]`,
			wantTemplate: `
path "my-db/creds/full" {
  capabilities = ["read"]
}`,
		},
		{
			name:     "test template func: githubAssignTeamPolicy",
			template: `[[ githubAssignTeamPolicy "my-team" "my-policy" ]]`,
			wantTemplate: `
secret "/auth/github/map/teams/my-team" {
  value = "my-policy"
}`,
		},
		{
			name:     "test template func: ldapAssignGroupPolicy",
			template: `[[ ldapAssignGroupPolicy "my-group" "my-policy" ]]`,
			wantTemplate: `
secret "/auth/ldap/groups/my-group" {
  value = "my-policy"
}`,
		},
		{
			name:     "test template func: grantCredentialsPolicy",
			template: `[[ grantCredentialsPolicy "my-db" "full" ]]`,
			wantTemplate: `
policy "my-db-full" {
  path "my-db/creds/full" {
    capabilities = ["read"]
  }
}`,
		},
		{
			name: "test template func: scratch",
			template: `[[ scratch.Set "foo" "bar" ]]
test = "[[ scratch.Get "foo" ]]"
`,
			wantTemplate: `test = "bar"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer, err := newRenderer(nil, nil)
			renderer.variables = tt.templateVariables
			require.NoError(t, err)

			got, err := renderer.renderContent(tt.template, "test", 0)
			if tt.wantErr != nil {
				require.True(t, strings.Contains(err.Error(), tt.wantErr.Error()))
				require.Equal(t, "", tt.wantTemplate, "you should not expect a template during error tests")
				return
			}

			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(tt.wantTemplate), got)
		})
	}
}
