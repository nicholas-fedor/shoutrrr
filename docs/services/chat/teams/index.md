# Teams

!!! Attention "Office 365 Connector Retirement"
    As noted in Microsoft's [update](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/), Office 365 Connectors within Microsoft Teams have been retired.

    The legacy `webhook.office.com` webhook URLs and MessageCard format are no longer functional.

    Shoutrrr now uses Power Automate workflow incoming webhooks exclusively.
    Existing configurations using the old URL format must be migrated to the new `teams://?host=WORKFLOW_URL` format documented below.

## URL Format

!!! info ""
    teams://?host=__`WORKFLOW_URL`__[&color=__`COLOR`__][&title=__`TITLE`__]

Where:

- `WORKFLOW_URL`: The full Power Automate workflow incoming webhook URL, percent-encoded. Each send needs one. A service URL may leave `host` out, and each send then has to provide it as a `host` param.
- `COLOR` *(optional)*: Title text color. Accepts Adaptive Card color names (`accent`, `good`, `warning`, `attention`, `dark`, `light`, `default`) or common color names (`red`, `green`, `blue`, `yellow`, `orange`).
- `TITLE` *(optional)*: Title displayed as a bold header in the Adaptive Card.

--8<-- "docs/services/chat/teams/config.md"

## Setting up a webhook

To use the Microsoft Teams notification service with Power Automate, create a new workflow in Power Automate and add the "When a Teams webhook request is received" trigger.
Copy the generated webhook URL from the trigger.

For more information, see the [Microsoft Learn documentation](https://learn.microsoft.com/en-us/connectors/teams/#microsoft-teams-webhook).

## Webhook URL

Shoutrrr checks the webhook URL when it reads the service URL, so `shoutrrr verify` reports a webhook URL it rejects.

### Accepted Hosts

The webhook URL must use one of the workflow hosts Microsoft issues:

| Platform                        | Host ends in                         |
|---------------------------------|--------------------------------------|
| Power Automate                  | `.environment.api.powerplatform.com` |
| Azure Logic Apps                | `.logic.azure.com`                   |
| Azure Logic Apps, US Government | `.logic.azure.us`                    |
| Azure Logic Apps, China         | `.logic.azure.cn`                    |

Retired `webhook.office.com` connector URLs are rejected.

!!! note "Power Automate URLs on `logic.azure.com`"
    Power Automate flows left `logic.azure.com` on November 30, 2025, and their old URLs on that domain no longer trigger them.
    Copy the current URL from the flow's trigger.

    Shoutrrr still accepts `logic.azure.com` URLs.
    An Azure Logic Apps URL has the same form as an old flow URL, so Shoutrrr cannot tell the two apart.

### What Shoutrrr Checks

The check covers these parts of the webhook URL:

- The scheme, which must be `https`.
- The host, which must be one of the accepted hosts.
- The path, up to `workflows/`.
- A non-empty `sig` signature in the query.

A workflow that anyone can trigger has a `sig` signature.
Shoutrrr cannot sign in to a workflow that requires sign-in.

A URL that passes the check can still fail to send, for example when its signature is wrong.

### Percent-encoding

Percent-encode the webhook URL in `host`, as the [example](#example) below does.
Without the encoding, the webhook URL's own parameters become separate keys of the service URL, which Shoutrrr rejects as unknown.

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
