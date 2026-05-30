# Teams

!!! Attention "Office 365 Connector Retirement"
    As noted in Microsoft's [update](https://devblogs.microsoft.com/microsoft365dev/retirement-of-office-365-connectors-within-microsoft-teams/),
    Office 365 Connectors within Microsoft Teams have been retired.

    The legacy `webhook.office.com` webhook URLs and MessageCard format are no longer functional.

    Shoutrrr now uses Power Automate workflow incoming webhooks exclusively.
    Existing configurations using the old URL format must be migrated to the new
    `teams://?host=<workflow URL>` format documented below.

## URL Format

```text
teams://?host=<Power Automate workflow URL>[&color=<hex color>][&title=<title>]
```

Where:

- `host`: The full Power Automate workflow incoming webhook URL (required).
- `color` *(optional)*: Title text color. Accepts Adaptive Card color names (`accent`, `good`, `warning`, `attention`, `dark`, `light`, `default`) or common color names (`red`, `green`, `blue`, `yellow`, `orange`).
- `title` *(optional)*: Title displayed as a bold header in the Adaptive Card.

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
https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX
```

Shoutrrr URL:

```text
teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2Fabc123%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-00%26sp%3D%2Ftriggers%2Fmanual%2Frun%26sv%3D1.0%26sig%3DXXXXXXXX&title=Alert
```

In this example:

- The `host` param is the full Power Automate workflow webhook URL (percent-encoded).
- `title=Alert` adds a bold title header to the Adaptive Card.
