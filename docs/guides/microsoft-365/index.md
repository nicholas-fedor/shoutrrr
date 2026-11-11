# Microsoft 365 Email

## Overview

This guide sets up the [SMTP](../../services/email/smtp/index.md) service to send email through Microsoft 365 (Exchange Online) with OAuth2.
Exchange Online is retiring Basic authentication for SMTP AUTH, so URLs that send a username and password stop working.

## Basic Authentication Retirement

Exchange Online turns off Basic authentication for SMTP AUTH by default in existing tenants at the end of December 2026.
Tenants created after that date don't have it.
After that, URLs for `smtp.office365.com` with `auth=Plain` or `auth=Login` stop working unless an administrator turns Basic authentication back on.
Microsoft plans to announce the final removal date in the second half of 2027.
For details, see Microsoft's [deprecation timeline](https://techcommunity.microsoft.com/blog/exchange/updated-exchange-online-smtp-auth-basic-authentication-deprecation-timeline/4489835).

## Choosing an OAuth2 Method

Shoutrrr supports two ways to get an OAuth2 access token for Microsoft 365.

- With [client credentials](../../services/email/smtp/index.md#oauth2_client_credentials), an application sends as a mailbox without a user's sign-in. The URL keeps working until the client secret expires.
- With a [refresh token](../../services/email/smtp/index.md#oauth2_token_refresh) from the [OAuth2 generator](../../usage/cli/generate/index.md#oauth2), a user signs in once. Refresh tokens expire after 90 days, so generate a new URL before then.

Client credentials suit a service that sends notifications unattended.

## Setting Up Client Credentials

These steps follow Microsoft's guide to [authenticating an SMTP connection with OAuth](https://learn.microsoft.com/en-us/exchange/client-developer/legacy-protocols/how-to-authenticate-an-imap-pop-smtp-application-by-using-oauth#use-client-credentials-grant-flow-to-authenticate-smtp-imap-and-pop-connections).

1. Register an application in Microsoft Entra ID and create a client secret for it.
2. Add the `SMTP.SendAsApp` application permission of Office 365 Exchange Online, and have an administrator grant consent.
3. Register the application's service principal in Exchange Online with `New-ServicePrincipal`.
4. Give the service principal access to the mailbox the URL's username names, with `Add-MailboxPermission -AccessRights FullAccess`.
5. When `fromaddress` differs from the username, give the username's mailbox (`MAILBOX`) SendAs permission on the `fromaddress` address (`FROM_ADDRESS`), with `Add-RecipientPermission -Identity FROM_ADDRESS -Trustee MAILBOX -AccessRights SendAs`.
6. Make sure [SMTP AUTH is turned on](https://learn.microsoft.com/en-us/exchange/clients-and-mobile-in-exchange-online/authenticated-client-smtp-submission) for the mailbox.

!!! Example "Microsoft 365 with client credentials"

    ```uri
    smtp://alerts%40example.com@smtp.office365.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&clientid=CLIENT_ID&clientsecret=CLIENT_SECRET&scopes=https%3A%2F%2Foutlook.office365.com%2F.default&tokenurl=https%3A%2F%2Flogin.microsoftonline.com%2FTENANT_ID%2Foauth2%2Fv2.0%2Ftoken&requirestarttls=yes
    ```

    Replace the following:

    - `CLIENT_ID`: the application (client) ID of the Entra application.
    - `CLIENT_SECRET`: its client secret, percent-encoded.
    - `TENANT_ID`: the directory (tenant) ID.

    The username is the sending mailbox, with its `@` written as `%40`.
    The scope `https://outlook.office365.com/.default` is the one Microsoft requires for SMTP.

## Using a Refresh Token

1. Register an application in Microsoft Entra ID with the redirect URI `http://localhost`, and create a client secret for it. Entra ignores the port of a `localhost` redirect URI, so the generator can listen on any free port.
2. Add the delegated `SMTP.Send` permission. The sending user consents when signing in, unless the tenant requires an administrator to grant consent.
3. Run the [OAuth2 generator](../../usage/cli/generate/index.md#oauth2) with `shoutrrr generate smtp oauth2 -x`, and answer its prompts with the values below. The `-x` flag prints the client secret and refresh token, which the generator otherwise masks.
4. Open the authorization URL it prints, and sign in as the sending user.
5. Change the port in the generated URL from 25 to 587, which Microsoft recommends for SMTP AUTH. Some networks and providers block port 25.
6. Store the URL as you would a password, since it holds the client secret and refresh token.

| Prompt        | Value                                                               |
|---------------|---------------------------------------------------------------------|
| AuthURL       | `https://login.microsoftonline.com/TENANT_ID/oauth2/v2.0/authorize` |
| TokenURL      | `https://login.microsoftonline.com/TENANT_ID/oauth2/v2.0/token`     |
| RedirectURL   | `http://localhost`                                                  |
| Scopes        | `https://outlook.office.com/SMTP.Send,offline_access`               |
| SMTP Hostname | `smtp.office365.com`                                                |

The `offline_access` scope makes Microsoft return a refresh token.
The [SMTP service page](../../services/email/smtp/index.md#oauth2_token_refresh) describes the settings the generator writes.
