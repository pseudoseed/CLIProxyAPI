# Account routing

Open `/routing.html` on the proxy server and connect with your management key. This built-in page is available independently of the downloaded `/management.html` application and uses the same authenticated management API. Filter by provider (Claude, Codex, etc.) to edit account priorities and Codex transport overrides. Runtime-only accounts cannot be edited here.

New configuration defaults to `routing.strategy: soonest-reset` and `routing.session-affinity: true`. Explicit settings in existing config files remain effective; use the page to enable the new policy for an existing installation.

An existing thread retains its available account, even if another account gains a higher priority or has an earlier reset. New threads and failover choose the highest available priority tier, then the earliest future subscription reset observed in upstream Claude or Codex response quota signals. Equal reset times rotate. When no candidate has a known future reset, selection falls back to round robin. Reset data is passive: a fresh account may have no observations until it receives a response. Other providers use the fallback policy.

Affinity uses existing session headers, conversation identifiers, prompt cache keys, and content-prefix matching. Clients should send a stable session identifier across turns. Bindings are in memory and default to a one-hour lifetime; restarting the proxy clears them.

Codex OAuth subscription accounts default to upstream WebSocket for streaming and non-streaming requests, including HTTP downstream clients. Explicit account `websockets: false` forces HTTP. API-key accounts retain their existing opt-in behavior. The existing executor handles compact requests and supported upgrade fallback; this change does not guarantee a latency improvement for every workload.
