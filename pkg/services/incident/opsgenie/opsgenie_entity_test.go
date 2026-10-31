package opsgenie

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitySetFromProp verifies that SetFromProp reads an OpsGenie ID into
// the ID of any entity type, a team, escalation, or schedule identifier into
// the name, and a user identifier into the username.
func TestEntitySetFromProp(t *testing.T) {
	t.Parallel()

	// tests lists entity props with the entity each should produce.
	tests := []struct {
		name string
		prop string
		want Entity
	}{
		{name: "team ID", prop: "team:" + testTeamID, want: Entity{Type: "team", ID: testTeamID}},
		{name: "team name", prop: "team:NOC", want: Entity{Type: "team", Name: "NOC"}},
		{name: "user ID", prop: "user:bb4d9938-c3c2-455d-aaab-727aa701c0d8", want: Entity{Type: "user", ID: "bb4d9938-c3c2-455d-aaab-727aa701c0d8"}},
		{name: "user username", prop: "user:trinity@opsgenie.com", want: Entity{Type: "user", Username: "trinity@opsgenie.com"}},
		{
			name: "escalation ID",
			prop: "escalation:aee8a0de-c80f-4515-a232-501c0bc9d715",
			want: Entity{Type: "escalation", ID: "aee8a0de-c80f-4515-a232-501c0bc9d715"},
		},
		{
			name: "escalation name",
			prop: "escalation:Nightwatch Escalation",
			want: Entity{Type: "escalation", Name: "Nightwatch Escalation"},
		},
		{
			name: "schedule name",
			prop: "schedule:First Responders Schedule",
			want: Entity{Type: "schedule", Name: "First Responders Schedule"},
		},
		{
			name: "schedule ID",
			prop: "schedule:80564037-1984-4f38-b98e-8a1f662df552",
			want: Entity{Type: "schedule", ID: "80564037-1984-4f38-b98e-8a1f662df552"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entity := Entity{}
			require.NoError(t, entity.SetFromProp(tt.prop))

			assert.Equal(t, tt.want, entity)
		})
	}
}

// TestEntitySetFromPropErrors verifies that SetFromProp rejects a prop without
// exactly one colon, an entity without an identifier, which Opsgenie requires
// for every responder and visibleTo entry, and a name for a type other than
// team or user.
func TestEntitySetFromPropErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid entity props with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		prop    string
	}{
		{name: "no colon", prop: "NOC", wantErr: ErrInvalidEntityFormat},
		{name: "two colons", prop: "user:a:b", wantErr: ErrInvalidEntityFormat},
		{name: "name for an unknown type", prop: "group:NOC", wantErr: ErrUnexpectedEntityType},
		{name: "user without a username", prop: "user:", wantErr: ErrMissingEntityIdentity},
		{name: "team without a name", prop: "team:", wantErr: ErrMissingEntityIdentity},
		{name: "escalation without a name", prop: "escalation:", wantErr: ErrMissingEntityIdentity},
		{name: "schedule without a name", prop: "schedule:", wantErr: ErrMissingEntityIdentity},
		{name: "no type and no identifier", prop: ":", wantErr: ErrMissingEntityIdentity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Entity{}).SetFromProp(tt.prop), tt.wantErr)
		})
	}
}

// TestEntityGetPropValue verifies that GetPropValue writes the type and the
// first set identifier, preferring the ID, then the name, then the username.
func TestEntityGetPropValue(t *testing.T) {
	t.Parallel()

	// tests lists entities with the prop each should produce.
	tests := []struct {
		name   string
		want   string
		entity Entity
	}{
		{name: "ID", entity: Entity{Type: "team", ID: testTeamID, Name: "NOC"}, want: "team:" + testTeamID},
		{name: "name", entity: Entity{Type: "team", Name: "NOC", Username: "ignored"}, want: "team:NOC"},
		{name: "username", entity: Entity{Type: "user", Username: "trinity@opsgenie.com"}, want: "user:trinity@opsgenie.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, err := tt.entity.GetPropValue()

			require.NoError(t, err)
			assert.Equal(t, tt.want, value)
		})
	}

	t.Run("no identifier", func(t *testing.T) {
		t.Parallel()

		value, err := (&Entity{Type: "team"}).GetPropValue()

		require.ErrorIs(t, err, ErrMissingEntityIdentity)
		assert.Empty(t, value)
	})
}

// TestOpsGenieIDPattern verifies that only lowercase hex IDs in the 8-4-4-4-12
// layout count as OpsGenie IDs.
func TestOpsGenieIDPattern(t *testing.T) {
	t.Parallel()

	// tests lists candidate strings and whether each is an ID.
	tests := []struct {
		input string
		want  bool
	}{
		{input: testTeamID, want: true},
		{input: "4513B7EA-3B91-438F-B7E4-E3E54AF9147C", want: false},
		{input: "4513b7ea3b91438fb7e4e3e54af9147c", want: false},
		{input: "NOC", want: false},
		{input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, opsGenieIDPattern.MatchString(tt.input))
		})
	}
}
