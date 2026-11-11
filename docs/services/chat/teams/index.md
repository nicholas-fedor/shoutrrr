# Teams

!!! Attention "Office 365 Connector Retirement"
    As noted in Microsoft's [update](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/),
    Office 365 Connectors within Microsoft Teams have been retired.

    The legacy `webhook.office.com` webhook URLs and MessageCard format are no longer functional.

    Shoutrrr now uses Power Automate workflow incoming webhooks exclusively.
    Existing configurations using the old URL format must be migrated to the new
    `teams://?host=WORKFLOW_URL` format documented below.

## URL Format

!!! info ""
    teams://?host=__`WORKFLOW_URL`__[&color=__`COLOR`__][&title=__`TITLE`__]

Where:

- `WORKFLOW_URL`: The full Power Automate workflow incoming webhook URL, percent-encoded. Each send needs one. A service URL may leave `host` out, and each send then has to provide it as a `host` param.
- `COLOR` *(optional)*: Title text color. Accepts Adaptive Card color names (`accent`, `good`, `warning`, `attention`, `dark`, `light`, `default`) or common color names (`red`, `green`, `blue`, `yellow`, `orange`).
- `TITLE` *(optional)*: Title displayed as a bold header in the Adaptive Card.

Shoutrrr checks `WORKFLOW_URL` when it reads the service URL, so `shoutrrr verify` reports a webhook URL that the check rejects.
The check covers the host, the path up to `workflows/`, and the presence of a signature.
A URL that passes can still fail to send, for example with a wrong signature.
The webhook URL must start like the workflow URLs Microsoft issues.
Power Automate uses an `environment.api.powerplatform.com` host.
Azure Logic Apps uses `logic.azure.com`, or `logic.azure.us` and `logic.azure.cn` in the US Government and China clouds.
Its query must carry a `sig` signature.
A workflow that anyone can trigger has one, and Shoutrrr cannot sign in to a workflow that requires sign-in.
Power Automate flows left `logic.azure.com` on November 30, 2025, and their old URLs on that domain no longer trigger them.
Copy the current URL from the flow's trigger.
Shoutrrr cannot tell an old flow URL from an Azure Logic Apps URL, which has the same form, so it accepts both.
Retired `webhook.office.com` connector URLs are rejected too.
Percent-encode the webhook URL in `host`, as the example below does.
Its parameters would otherwise become separate keys of the service URL, which Shoutrrr rejects as unknown.

--8<-- "docs/services/chat/teams/config.md"

## Setting up a webhook

To use the Microsoft Teams notification service with Power Automate, create a new
workflow in Power Automate and add the "When a Teams webhook request is received"
trigger. Copy the generated webhook URL from the trigger.

For more information, see the
[Microsoft Learn documentation](https://learn.microsoft.com/en-us/connectors/teams/#microsoft-teams-webhook).

## Example

Teams/Power Automate workflow webhook URL:

```text
https://ENVIRONMENT.environment.api.powerplatform.com:443/powerautomate/automations/direct/workflows/WORKFLOW_ID/triggers/manual/paths/invoke?api-version=1&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=SIGNATURE
```

Shoutrrr URL:

```text
teams://?host=https%3A%2F%2FENVIRONMENT.environment.api.powerplatform.com%3A443%2Fpowerautomate%2Fautomations%2Fdirect%2Fworkflows%2FWORKFLOW_ID%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D1%26sp%3D%252Ftriggers%252Fmanual%252Frun%26sv%3D1.0%26sig%3DSIGNATURE&title=Alert
```

In this example:

- The `host` param is the full Power Automate workflow webhook URL (percent-encoded).
- `title=Alert` adds a bold title header to the Adaptive Card.
