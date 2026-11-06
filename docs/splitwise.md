# Splitwise (`dieuctl splitwise`)

Local CLI for your own Splitwise account. It calls the same `/api/v3.0` JSON API the dashboard at `https://secure.splitwise.com/#/dashboard` already uses. It does **not** dump Chrome cookies, attach to a Cloud Agent, or store secrets in the cluster.

## Auth

First match wins:

1. **Personal API key** (if Splitwise still lets your account mint one): 1Password item `splitwise.api-token`, field `credential`.
2. **Dashboard session** (free account): 1Password item `splitwise.session`, fields `cookie` (the request `Cookie` header) and `csrf_token` (`X-CSRF-Token`).

`~/.config/dieuctl/config.yaml` holds only `op://` refs:

```yaml
splitwise:
  api_token: "op://1Password/splitwise.api-token/credential"   # optional
  session: "op://1Password/splitwise.session/cookie"
  csrf: "op://1Password/splitwise.session/csrf_token"
```

Treat the cookie like a password. Never paste it into chat, git, or Kubernetes.

`DIEUCTL_SPLITWISE_BASE_URL` overrides the API base (tests and a local mock). Leave it unset for `https://secure.splitwise.com/api/v3.0`.

### Export a session (once, on your laptop)

1. Open `https://secure.splitwise.com/#/dashboard` while logged in.
2. DevTools → Network → any `get_expenses` / `get_current_user` call.
3. Copy the request `Cookie` header and `X-CSRF-Token`.
4. Paste into 1Password item `splitwise.session`. If requests start 401/302, refresh those fields.

## Commands

```bash
dieuctl splitwise whoami
dieuctl splitwise groups
dieuctl splitwise expenses --group Apartment
dieuctl splitwise add --cost 42.00 --description Groceries --group Apartment
dieuctl splitwise add --cost 42.00 --description Groceries --group Apartment \
  --share 111:42.00:21.00 --share 222:0:21.00
```

`--group` accepts a numeric id or a case-insensitive name.

## Local Cursor agent

A **local** Cursor agent on your laptop can run these commands in the terminal (`dieuctl splitwise add …`). That is the reliable path.

It does **not** inherit your daily Chrome Splitwise tab.

| How you run the agent | Splitwise login |
| --- | --- |
| Local Cursor Agent + `dieuctl` (recommended) | 1Password session or API key, as above |
| Local Cursor Agent + [built-in Browser](https://cursor.com/docs/agent/tools/browser) | Isolated Chromium; cookies persist per workspace. Log into Splitwise once *in Cursor’s browser*, then click the dashboard. |
| Real-Chrome MCP you install yourself | Can drive the tab you already have open. Use a dedicated Chrome profile. Do not configure it to exfiltrate cookies. |
| This Cloud Agent / hosted Agent SDK | Cannot see your laptop browser. Do not give it the cookie. |

Example local-agent prompt after `dieuctl` works:

> add $42 groceries to Apartment, split equally

The agent should run `dieuctl splitwise add --cost 42.00 --description Groceries --group Apartment`, not scrape Chrome’s cookie database.
