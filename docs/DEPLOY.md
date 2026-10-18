# Hosting the web app

ebb is not a service. It runs on each Mac. The only part that can be hosted is the window: static files that run in the visitor's browser and talk to ebb on that visitor's own machine, at `127.0.0.1`. The host holds no data and no secrets.

## Build

```sh
cd ui && bun install --frozen-lockfile && bun run build
```

The output goes to `internal/server/assets/dist`, the same files ebb embeds. Its `_headers` file sets the content security policy, `nosniff` and no referrer. ebb serves the same rules from that file, so they live in one place.

## Cloudflare

ebb's own copy runs as a Worker with static assets, set up in `web/`. Cloudflare's `cf` CLI runs on Node.js only, so
that folder uses npm:

```sh
cd ui && bun install --frozen-lockfile && bun run build
cd ../web && npm ci && npx cf deploy
```

`web/wrangler.config.ts` points at the same build the Go binary embeds. `web/cloudflare.config.ts` names the Worker,
answers unknown paths with `index.html` and serves it at `ebb.apps.doguyilmaz.com`. Cloudflare reads `_headers` from
the build.

Releases deploy on their own through Workers Builds, so no Cloudflare token lives in GitHub. The release workflow moves
the `release` branch to the new tag once the binaries are out. Workers Builds then deploys that branch. The page never
runs ahead of the ebb people can install. When it is newer than the ebb it talks to, it says so and shows
`brew upgrade ebb`. The Worker's build settings in the dashboard are:

| Setting | Value |
| --- | --- |
| Branch | `release` only (no trigger for other branches) |
| Watch paths | `ui/*` and `web/*` (Go, docs and rules changes do not deploy) |
| Path | `/web` |
| Build command | `git fetch --quiet --unshallow --tags origin; cd ../ui && bun install --frozen-lockfile && bun run build && cd ../web && npm ci` |
| Deploy command | `npx cf deploy` |
| Variable | `BUN_VERSION=1.4.3` |

Workers Builds clones without history or tags. The fetch brings them back so the page can read its version from the
latest tag.

The commands above are for deploying by hand, which is only needed when Workers Builds is down.

Any other static host works too if it sends the rules in `_headers` and answers every unknown path with `index.html`.

## Letting a machine use it

ebb trusts `https://ebb.apps.doguyilmaz.com` out of the box. A copy on your own domain must be allowed once on each machine:

```sh
ebb web allow https://ebb.example.com
```

The first visit shows a pairing code. Confirm it at the machine, either with `ebb web confirm <code>` or in the local window. The page then keeps a token of its own and can do everything the local window can. `ebb web deny <origin>` removes the origin and every token it was given. That works for the built-in origin too.

## Limits

- **Safari** does not let an https page reach `http://127.0.0.1`. Use Chrome, Edge or Firefox. The local window (`ebb serve --open`) works in any browser.
- **Chrome** may ask once whether the site may reach devices on your local network. Without that permission the page cannot find ebb.
- **The content security policy** allows only the bundle's own scripts. That also blocks extensions that inject into the page, React DevTools among them.
