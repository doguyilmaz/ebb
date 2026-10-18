# Hosting the web app

ebb is not a service. It runs on each Mac. The only part that can be hosted is the window: a static page that runs in the visitor's browser and talks to ebb on that visitor's own machine. The server holds no data and no secrets. Two people opening the same address each see their own disk.

## Build and run

```sh
docker build -t ebb-web .
docker run -p 8080:8080 ebb-web
```

The image runs `ebb-web`, which serves the built page with the same headers as `ebb serve`: the content security policy, `nosniff` and no referrer. It listens on `--listen` or `EBB_WEB_LISTEN`. The default is `127.0.0.1:8080`, and the image uses `0.0.0.0:8080`. It does not terminate TLS, so put a proxy in front of it.

## Coolify

1. Create an application from this repository and choose the **Dockerfile** build pack.
2. Set **Ports Exposes** to `8080` and give it a domain. Coolify's proxy handles the certificate.
3. Deploy.

It needs no environment variables and no volume, and the image carries its own health check.

## Letting a machine use it

ebb trusts `https://ebb.apps.doguyilmaz.com` out of the box. A copy on your own domain must be allowed once on each machine:

```sh
ebb web allow https://ebb.example.com
```

The first visit shows a pairing code. Confirm it at the machine, either with `ebb web confirm <code>` or in the local window. The page then keeps a token of its own. `ebb web deny <origin>` removes the origin and every token it was given, and that works for the built-in origin too.

## Limits

- **Safari** does not let an https page reach `http://127.0.0.1`. Use Chrome, Edge or Firefox, or the local window (`ebb serve --open`).
- **Chrome** may ask once whether the site may reach devices on your local network. Allow it, or the page cannot find ebb.
- **The content security policy** allows only the bundle's own scripts. That also blocks extensions that inject into the page, React DevTools among them.
