package opsgenie

// AlertPayload represents the payload being sent to the OpsGenie API
//
// See: https://docs.opsgenie.com/docs/alert-api#create-alert
//
// Some fields hold lists and maps. Params are strings, so these fields are
// set from comma-separated values: responders and visibleTo as type:identifier
// entries, actions and tags as lists, and details as key:value pairs:
//
//	service.Send("An example alert message", &types.Params{
//		"alias":       "Life is too short for no alias",
//		"description": "Every alert needs a description",
//		"responders":  "team:4513b7ea-3b91-438f-b7e4-e3e54af9147c,team:NOC,escalation:Nightwatch Escalation",
//		"visibleTo":   "team:rocket_team,user:trinity@opsgenie.com",
//		"tags":        "OverwriteQuietHours,Critical",
//		"details":     "key1:value1,key2:value2",
//	})
type AlertPayload struct {
	Details     map[string]string `json:"details,omitempty"`
	Message     string            `json:"message"`
	Alias       string            `json:"alias,omitempty"`
	Description string            `json:"description,omitempty"`
	Entity      string            `json:"entity,omitempty"`
	Source      string            `json:"source,omitempty"`
	Priority    string            `json:"priority,omitempty"`
	User        string            `json:"user,omitempty"`
	Note        string            `json:"note,omitempty"`
	Responders  []Entity          `json:"responders,omitempty"`
	VisibleTo   []Entity          `json:"visibleTo,omitempty"`
	Actions     []string          `json:"actions,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}
