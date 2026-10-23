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
	// Details holds custom properties of the alert.
	Details map[string]string `json:"details,omitempty"`
	// Message is the alert message, at most [MaxMessageLength] characters.
	Message string `json:"message"`
	// Alias is the client-defined identifier used to deduplicate alerts.
	Alias string `json:"alias,omitempty"`
	// Description is the alert's longer description.
	Description string `json:"description,omitempty"`
	// Entity names the domain of the alert's source.
	Entity string `json:"entity,omitempty"`
	// Source is the source of the alert.
	Source string `json:"source,omitempty"`
	// Priority is the alert priority, P1 to P5.
	Priority string `json:"priority,omitempty"`
	// User is the display name of the request owner.
	User string `json:"user,omitempty"`
	// Note is added to the alert when it is created.
	Note string `json:"note,omitempty"`
	// Responders lists the teams, users, escalations, and schedules the alert
	// is routed to.
	Responders []Entity `json:"responders,omitempty"`
	// VisibleTo lists the teams and users that can see the alert without being
	// notified.
	VisibleTo []Entity `json:"visibleTo,omitempty"`
	// Actions lists the custom actions available on the alert.
	Actions []string `json:"actions,omitempty"`
	// Tags lists the alert's tags.
	Tags []string `json:"tags,omitempty"`
}
