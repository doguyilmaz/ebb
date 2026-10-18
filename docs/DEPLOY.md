# Hosting the web app

ebb is not a service. It runs on each Mac. The only part that can be hosted is the window: static files that run in the visitor's browser and talk to ebb on that visitor's own machine, at `127.0.0.1`. The host holds no data and no secrets.

## Build

```sh
cd ui && bun install --frozen-lockfile && bun run build
```

The output goes to `internal/server/assets/dist`, the same files ebb embeds. Its `_headers` file sets the content security policy, `nosniff` and no referrer. ebb serves the same rules from that file, so they live in one place.

## Cloudflare

Any static host works if it can do two things:

- send the rules in `_headers`;
- answer every unknown path with `index.html`, because the app does its own routing.

Cloudflare reads `_headers` either way:

- **Workers static assets:** set `assets.directory` to `internal/server/assets/dist` and `assets.not_found_handling` to `single-page-application`.
- **Pages:** use the build command above with output directory `internal/server/assets/dist`. With no `404.html`, Pages answers unknown paths with `index.html` on its own.

## Letting a machine use it

ebb trusts `https://ebb.apps.doguyilmaz.com` out of the box. A copy on your own domain must be allowed once on each machine:

```sh
ebb web allow https://ebb.example.com
```

The first visit shows a pairing code. Confirm it at the machine, either with `ebb web confirm <code>` or in the local window. The page then keeps a token of its own and can do everything the local window can. `ebb web deny <origin>` removes the origin and every token it was given, and that works for the built-in origin too.

## Limits

- **Safari** does not let an https page reach `http://127.0.0.1`. Use Chrome, Edge or Firefox, or the local window (`ebb serve --open`).
- **Chrome** may ask once whether the site may reach devices on your local network. Allow it, or the page cannot find ebb.
- **The content security policy** allows only the bundle's own scripts. That also blocks extensions that inject into the page, React DevTools among them.
